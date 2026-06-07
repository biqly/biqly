/** Base64url credential descriptor from Biqly auth API (before buffer conversion). */
export interface WebAuthnCredentialDescriptorJSON {
  id: string
  type?: PublicKeyCredentialType
  transports?: AuthenticatorTransport[]
}

export interface PasskeyRequestOptionsJSON {
  challenge: string
  allowCredentials?: WebAuthnCredentialDescriptorJSON[]
  excludeCredentials?: WebAuthnCredentialDescriptorJSON[]
  timeout?: number
  rpId?: string
  userVerification?: UserVerificationRequirement
  user?: { id: string; name?: string; displayName?: string }
}

/** Registration begin payload from auth API (base64url fields before buffer conversion). */
export interface PasskeyCreationOptionsJSON extends PasskeyRequestOptionsJSON {
  rp: PublicKeyCredentialRpEntity
  pubKeyCredParams: PublicKeyCredentialParameters[]
  authenticatorSelection?: AuthenticatorSelectionCriteria
  attestation?: AttestationConveyancePreference
}

export function bufferToBase64url(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]!)
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '')
}

export function base64urlToBuffer(base64url: string): ArrayBuffer {
  const padding = '='.repeat((4 - (base64url.length % 4)) % 4)
  const base64 = (base64url + padding).replace(/-/g, '+').replace(/_/g, '/')
  const binary = atob(base64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes.buffer
}
