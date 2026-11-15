# Working in servterm

Ship only changes that advance the requested task. Keep the diff small, and
stop when its acceptance criteria are met.

- Read the relevant code before editing; preserve unrelated work.
- Run only the checks the change can break. The CIP queue runs the full gate.
- Fail at the first error. Diagnose a repeated failure instead of retrying it.
- Do not skip or weaken a failing check to get a green result.
- Lease the CIP task before editing, record evidence on the task, and release
  the lease when handing work off.
- Push changes to the native CIP forge, link the change to its task, and enqueue
  it. The queue and promotion perform merge and deployment.
- A merge is not complete until the deployed bytes are verified on the target.
- Do not rebuild binaries on deployment hosts; deploy the artifacts produced by
  the pipeline.
- Preserve user data and existing host configuration. Stop and report if a
  deployment would require an irreversible change or an unapproved credential.
