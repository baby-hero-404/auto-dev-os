# node runtime image: autocode-base already carries Node 22 (needed by the
# CLI agents themselves), so there is nothing additional to install here
# today. Kept as its own layer (rather than pointing the node manifest
# straight at autocode-base) so a future node-specific addition (e.g. a
# global lint/test tool every node project sandbox should have) has a place
# to go without touching the shared base.
FROM autocode-base:latest
