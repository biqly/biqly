import { useState } from 'react'

import { apiPasskeyRegisterBegin, apiPasskeyRegisterFinish } from '../api/auth'
import {
  base64urlToBuffer,
  bufferToBase64url,
  resolvePasskeyRegisterOptions,
} from '../utils/webauthn'

export function usePasskeyRegistration(accessToken: string) {
  const [registering, setRegistering] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const registerPasskey = async (name: string): Promise<boolean> => {
    const isSupported =
      typeof window.PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable === 'function'

    if (!isSupported) {
      setError('passkeys.error_not_supported')
      return false
    }

    setRegistering(true)
    setError(null)

    try {
      // 1. Begin Registration on Backend
      const beginResp = await apiPasskeyRegisterBegin(accessToken)
      const publicKeyOptions = resolvePasskeyRegisterOptions(beginResp)

      const user = publicKeyOptions.user
      if (!publicKeyOptions.challenge || !user?.id) {
        throw new Error('Invalid options from server')
      }

      const options: CredentialCreationOptions = {
        publicKey: {
          rp: publicKeyOptions.rp,
          pubKeyCredParams: publicKeyOptions.pubKeyCredParams,
          challenge: base64urlToBuffer(publicKeyOptions.challenge),
          user: {
            id: base64urlToBuffer(user.id),
            name: user.name ?? 'user',
            displayName: user.displayName ?? user.name ?? 'Passkey',
          },
          timeout: publicKeyOptions.timeout,
          authenticatorSelection: publicKeyOptions.authenticatorSelection,
          attestation: publicKeyOptions.attestation,
          excludeCredentials: publicKeyOptions.excludeCredentials?.map((cred) => ({
            type: cred.type ?? 'public-key',
            id: base64urlToBuffer(cred.id),
            transports: cred.transports,
          })),
        },
      }

      // 2. Trigger browser's WebAuthn prompt
      const credential = await navigator.credentials.create(options)
      if (!credential) {
        throw new Error('No credential returned by browser')
      }

      // 3. Serialize response back to base64url
      const attestation = credential as PublicKeyCredential
      const response = attestation.response as AuthenticatorAttestationResponse
      const credentialJson = {
        id: attestation.id,
        rawId: bufferToBase64url(attestation.rawId),
        type: attestation.type,
        response: {
          clientDataJSON: bufferToBase64url(response.clientDataJSON),
          attestationObject: bufferToBase64url(response.attestationObject),
          transports: response.getTransports(),
        },
      }

      // 4. Finish registration on Backend
      await apiPasskeyRegisterFinish(accessToken, credentialJson, name.trim())
      return true
    } catch (err: unknown) {
      const errorObject = err as Record<string, any>
      if (errorObject.name !== 'NotAllowedError') {
        setError(errorObject.message ?? 'Passkey registration failed')
      }
      return false
    } finally {
      setRegistering(false)
    }
  }

  return { registering, error, setError, registerPasskey }
}
