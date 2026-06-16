#!/usr/bin/env node

import { spawn } from 'node:child_process'
import { existsSync, promises as fs } from 'node:fs'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(scriptDir, '..')
const workspaceDir = path.resolve(rootDir, '..')
const serverBin = path.join(rootDir, 'node_modules', '.bin', 'tailwindcss-language-server')
const defaultTimeoutMs = 12_000
const ignoredDirs = new Set(['.git', 'build', 'coverage', 'dist', 'node_modules'])
const supportedExtensions = new Set(['.css', '.html', '.js', '.jsx', '.mjs', '.ts', '.tsx'])
const severityName = new Map([
  [1, 'error'],
  [2, 'warning'],
  [3, 'info'],
  [4, 'hint'],
])

function parseArgs(args) {
  const parsed = {
    files: [],
    maxWarnings: Number.POSITIVE_INFINITY,
    timeoutMs: defaultTimeoutMs,
  }

  for (let index = 0; index < args.length; index += 1) {
    const arg = args[index]
    if (arg === '--max-warnings') {
      const value = args[index + 1]
      if (value == null || Number.isNaN(Number(value))) {
        throw new Error('--max-warnings expects a number')
      }
      parsed.maxWarnings = Number(value)
      index += 1
      continue
    }
    if (arg === '--timeout-ms') {
      const value = args[index + 1]
      if (value == null || Number.isNaN(Number(value))) {
        throw new Error('--timeout-ms expects a number')
      }
      parsed.timeoutMs = Number(value)
      index += 1
      continue
    }
    parsed.files.push(path.resolve(rootDir, arg))
  }

  return parsed
}

async function workspaceTailwindSettings() {
  const settingsPath = path.join(workspaceDir, '.vscode', 'settings.json')
  if (!existsSync(settingsPath)) {
    return {}
  }

  try {
    return JSON.parse(await fs.readFile(settingsPath, 'utf8'))
  } catch {
    return {}
  }
}

function tailwindSettingsFromVSCode(settings) {
  const lint = {}
  const prefix = 'tailwindCSS.lint.'

  for (const [key, value] of Object.entries(settings)) {
    if (key.startsWith(prefix)) {
      lint[key.slice(prefix.length)] = value
    }
  }

  return {
    validate: settings['tailwindCSS.validate'] ?? true,
    experimental: {
      configFile: settings['tailwindCSS.experimental.configFile'] ?? './src/index.css',
    },
    lint: {
      suggestCanonicalClasses: 'error',
      ...lint,
    },
  }
}

function languageIdFor(filePath) {
  switch (path.extname(filePath)) {
    case '.css':
      return 'tailwindcss'
    case '.html':
      return 'html'
    case '.jsx':
      return 'javascriptreact'
    case '.js':
    case '.mjs':
      return 'javascript'
    case '.tsx':
      return 'typescriptreact'
    case '.ts':
      return 'typescript'
    default:
      return 'plaintext'
  }
}

async function collectFiles(dir, files = []) {
  const entries = await fs.readdir(dir, { withFileTypes: true })
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      if (!ignoredDirs.has(entry.name)) {
        await collectFiles(fullPath, files)
      }
      continue
    }
    if (entry.isFile() && supportedExtensions.has(path.extname(entry.name))) {
      files.push(fullPath)
    }
  }
  return files
}

function createLSPClient(child, settings, openedUris) {
  let nextID = 1
  let buffer = Buffer.alloc(0)
  const pending = new Map()
  const diagnostics = new Map()

  function write(payload) {
    const body = JSON.stringify(payload)
    child.stdin.write(`Content-Length: ${Buffer.byteLength(body, 'utf8')}\r\n\r\n${body}`)
  }

  function request(method, params) {
    const id = nextID
    nextID += 1
    write({ jsonrpc: '2.0', id, method, params })
    return new Promise((resolve, reject) => {
      pending.set(id, { resolve, reject })
    })
  }

  function notify(method, params) {
    write({ jsonrpc: '2.0', method, params })
  }

  function respond(id, result) {
    write({ jsonrpc: '2.0', id, result })
  }

  function onMessage(message) {
    if (message.method === 'textDocument/publishDiagnostics') {
      diagnostics.set(message.params.uri, message.params.diagnostics ?? [])
      return
    }

    if (message.method === 'window/logMessage' && process.env.TAILWIND_LINT_VERBOSE === '1') {
      console.error(message.params.message)
      return
    }

    if (message.id != null && message.method === 'workspace/configuration') {
      respond(
        message.id,
        (message.params?.items ?? []).map((item) =>
          item.section === 'tailwindCSS' ? settings : null,
        ),
      )
      return
    }

    if (message.id != null && pending.has(message.id)) {
      const deferred = pending.get(message.id)
      pending.delete(message.id)
      if (message.error) {
        deferred.reject(new Error(message.error.message))
      } else {
        deferred.resolve(message.result)
      }
      return
    }

    if (message.id != null) {
      respond(message.id, null)
    }
  }

  child.stdout.on('data', (chunk) => {
    buffer = Buffer.concat([buffer, chunk])

    while (true) {
      const headerEnd = buffer.indexOf('\r\n\r\n')
      if (headerEnd === -1) {
        break
      }

      const header = buffer.subarray(0, headerEnd).toString('utf8')
      const lengthMatch = /^Content-Length:\s*(\d+)/im.exec(header)
      if (!lengthMatch) {
        throw new Error(`Invalid language-server frame header: ${header}`)
      }

      const length = Number(lengthMatch[1])
      const messageStart = headerEnd + 4
      const messageEnd = messageStart + length
      if (buffer.length < messageEnd) {
        break
      }

      const body = buffer.subarray(messageStart, messageEnd).toString('utf8')
      buffer = buffer.subarray(messageEnd)
      onMessage(JSON.parse(body))
    }
  })
  child.stdin.on('error', (error) => {
    if (error.code !== 'EPIPE') {
      throw error
    }
  })

  return {
    diagnostics,
    notify,
    openedUris,
    request,
  }
}

function formatDiagnostic(uri, diagnostic) {
  const filePath = fileURLToPath(uri)
  const relativePath = path.relative(workspaceDir, filePath)
  const line = diagnostic.range.start.line + 1
  const column = diagnostic.range.start.character + 1
  const severity = severityName.get(diagnostic.severity) ?? 'unknown'
  const code = diagnostic.code == null ? '' : ` ${diagnostic.code}`
  return `${relativePath}:${line}:${column} ${severity}${code} ${diagnostic.message}`
}

async function main() {
  const args = parseArgs(process.argv.slice(2))

  if (!existsSync(serverBin)) {
    throw new Error(
      'Missing tailwindcss-language-server. Run `npm --prefix frontend install` first.',
    )
  }

  const files = args.files.length > 0 ? args.files : await collectFiles(rootDir)
  const workspaceSettings = await workspaceTailwindSettings()
  const tailwindSettings = tailwindSettingsFromVSCode(workspaceSettings)
  const child = spawn(serverBin, ['--stdio'], {
    cwd: rootDir,
    stdio: ['pipe', 'pipe', 'inherit'],
  })
  const openedUris = new Set()
  const client = createLSPClient(child, tailwindSettings, openedUris)

  await client.request('initialize', {
    processId: process.pid,
    rootPath: rootDir,
    rootUri: pathToFileURL(rootDir).href,
    capabilities: {
      textDocument: {
        publishDiagnostics: {
          relatedInformation: true,
        },
      },
      workspace: {
        configuration: true,
        didChangeWatchedFiles: {
          dynamicRegistration: true,
        },
        didChangeConfiguration: {
          dynamicRegistration: false,
        },
      },
    },
    workspaceFolders: [
      {
        uri: pathToFileURL(rootDir).href,
        name: 'frontend',
      },
    ],
    initializationOptions: {
      testMode: true,
    },
  })

  client.notify('initialized', {})
  client.notify('workspace/didChangeConfiguration', {
    settings: {
      tailwindCSS: tailwindSettings,
    },
  })

  for (const filePath of files) {
    const uri = pathToFileURL(filePath).href
    const text = await fs.readFile(filePath, 'utf8')
    openedUris.add(uri)
    client.notify('textDocument/didOpen', {
      textDocument: {
        uri,
        languageId: languageIdFor(filePath),
        version: 1,
        text,
      },
    })
    client.notify('textDocument/didChange', {
      textDocument: {
        uri,
        version: 2,
      },
      contentChanges: [
        {
          text,
        },
      ],
    })
  }

  await new Promise((resolve) => {
    setTimeout(resolve, args.timeoutMs)
  })

  for (const uri of openedUris) {
    client.notify('textDocument/didClose', {
      textDocument: { uri },
    })
  }
  await client.request('shutdown', null)
  client.notify('exit', {})

  const allDiagnostics = []
  for (const [uri, items] of client.diagnostics.entries()) {
    for (const item of items) {
      allDiagnostics.push({ uri, diagnostic: item })
    }
  }

  allDiagnostics.sort((left, right) =>
    formatDiagnostic(left.uri, left.diagnostic).localeCompare(
      formatDiagnostic(right.uri, right.diagnostic),
    ),
  )

  for (const item of allDiagnostics) {
    console.log(formatDiagnostic(item.uri, item.diagnostic))
  }

  const errors = allDiagnostics.filter((item) => item.diagnostic.severity === 1).length
  const warnings = allDiagnostics.filter((item) => item.diagnostic.severity === 2).length
  const summary = `Tailwind diagnostics: ${errors} error(s), ${warnings} warning(s)`
  if (errors > 0 || warnings > args.maxWarnings) {
    console.error(summary)
    process.exitCode = 1
    return
  }

  console.log(summary)
}

main().catch((error) => {
  console.error(error.message)
  process.exitCode = 1
})
