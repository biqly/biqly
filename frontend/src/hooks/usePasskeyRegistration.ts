import { useState } from 'react'
import { apiPasskeyRegisterBegin, apiPasskeyRegisterFinish } from '../api/auth'
import { base64urlToBuffer, bufferToBase64url } from '../utils/webauthn'

export function usePasskeyRegistration(accessToken: string) {
  const [registering, setRegistering] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const registerPasskey = async (name: string): Promise<boolean> => {
    const isSupported = window.PublicKeyCredential &&
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
      const publicKeyOptions = beginResp.publicKey || beginResp

      if (!publicKeyOptions) {
        throw new Error('Invalid options from server')
      }

      // Convert challenge, user id, and excluded credentials IDs to ArrayBuffer
      const options: CredentialCreationOptions = {
        publicKey: {
          ...publicKeyOptions,
          challenge: base64urlToBuffer(publicKeyOptions.challenge),
          user: {
            ...publicKeyOptions.user,
            id: base64urlToBuffer(publicKeyOptions.user.id),
          },
          excludeCredentials: publicKeyOptions.excludeCredentials?.map((cred: any) => ({
            ...cred,
            id: base64urlToBuffer(cred.id),
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
          transports: response.getTransports ? response.getTransports() : [],
        },
      }

      // 4. Finish registration on Backend
      await apiPasskeyRegisterFinish(accessToken, credentialJson, name.trim())
      return true
    } catch (err: any) {
      if (err.name !== 'NotAllowedError') {
        setError(err.message || 'Passkey registration failed')
      }
      return false
    } finally {
      setRegistering(false)
    }
  }

  return { registering, error, setError, registerPasskey }
}
