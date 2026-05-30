# Settings Security Component Split Design

## Scope

Split the security-related presentation in `frontend/src/components/Settings.tsx`
into focused components without changing WebAuthn registration, passkey
management, MFA enrollment, MFA disable, MFA recovery-code regeneration, QR
generation, clipboard, alert, or API behavior.

The parent Settings component continues to own API calls, authentication state,
modal orchestration, loading state, success messages, and errors. The extracted
components stay controlled and presentational.

## Components

### PasskeyTable

`frontend/src/components/settings/PasskeyTable.tsx` renders passkey loading,
empty, and table states. Rename and delete actions are callbacks supplied by the
parent.

### MFASection

`frontend/src/components/settings/MFASection.tsx` renders MFA status, enable,
disable, and recovery-code regeneration controls. The parent retains all API
calls and modal state.

### RecoveryCodesDisplay

`frontend/src/components/settings/RecoveryCodesDisplay.tsx` renders recovery
codes and preserves the existing clipboard-copy interaction. It supports both
the enrollment modal and the post-regeneration inline display.

### OTPCodeInput

`frontend/src/components/settings/OTPCodeInput.tsx` renders the shared six-digit
OTP field. A small pure helper normalizes values to numeric characters and a
maximum length of six so the behavior can be covered by Vitest.

## Data Flow

`Settings.tsx` remains the stateful boundary:

1. API handlers update parent state.
2. Parent state is passed into controlled children.
3. Child callbacks notify the parent about user intent.
4. Existing modals remain in the parent and reuse the OTP and recovery-code
   components where applicable.

## Error Handling

Existing parent-level error handling remains unchanged. This refactor does not
alter API response interpretation or introduce new user-facing states.

## Verification

- Add focused tests for OTP normalization.
- Run the complete frontend Vitest suite.
- Run the production Vite build.
- Run `git diff --check`.
- Perform browser verification when the local browser runtime is available.
