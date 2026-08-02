# Campaign-1 — VOIDED, not experimental data

Halted after 6/20 slots (2026-08-02) due to a harness defect: `dojo --project X`
does not change the wrapped command's cwd, so 5 delegates' real work landed
outside the persisted/discoverable directory and was misclassified as
delivery FAIL. Slot 6 was mid-dispatch when the run hit an external timeout
(verified no surviving process; recorded infra-void).

Retained here for transparency and root-cause audit ONLY. The FAIL labels on
slots 1-5 are NOT trustworthy (true delegate performance unknown) and MUST
NOT be blended with campaign-2's data. Full record: jeeves tasks.jeeves
event #95146 (`/loopback 3a->3a`). Fix: `../dispatch.bash`'s dojo invocation
now `cd`s explicitly into the slot dir; verified via pre-flight step 2/7.
