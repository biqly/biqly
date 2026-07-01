# Triage Labels

The skills speak in terms of five canonical triage roles. This file maps those roles to the label strings this repo intends to use in GitHub.

| Label in mattpocock/skills | Label in our tracker | Meaning                                  |
| -------------------------- | -------------------- | ---------------------------------------- |
| `needs-triage`             | `needs-triage`       | Maintainer needs to evaluate this issue  |
| `needs-info`               | `needs-info`         | Waiting on reporter for more information |
| `ready-for-agent`          | `ready-for-agent`    | Fully specified, ready for an AFK agent  |
| `ready-for-human`          | `ready-for-human`    | Requires human implementation            |
| `wontfix`                  | `wontfix`            | Will not be actioned                     |

When a skill mentions a role (e.g. "apply the AFK-ready triage label"), use the corresponding label string from this table.

As of 2026-07-01, GitHub already has `wontfix`; `needs-triage`, `needs-info`, `ready-for-agent`, and `ready-for-human` are not present yet and must be created before use.
