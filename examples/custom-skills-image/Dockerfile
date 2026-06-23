# Custom skills image — example
#
# This is the runnable counterpart of the "Use case: Custom skills image via
# your own skills repo" section of the top-level README. Clone this directory
# into a new git repo (or copy it into your existing skills repo), then build:
#
#   podman build -t yourorg/skillpack-skills .
#
# The build does three things:
#   1. Starts FROM the published skillpack runtime image (distroless, ~11 MB).
#   2. Copies your skills/ and commands/ trees on top, replacing the embedded
#      example skills.
#   3. Defaults the container to serve those trees over HTTP.
#
# To keep the embedded example skills (markdown-lint, commit) alongside your
# own, switch the CMD to use --merge-skills / --merge-commands instead of the
# bare --skills-dir / --commands-dir flags (see the top-level README).

FROM ghcr.io/marstid/skillpack:latest

# Replace the embedded trees entirely with your own.
COPY skills/   /skills/
COPY commands/ /commands/

# Serve the custom trees over HTTP. Use --merge-skills / --merge-commands here
# instead of the bare --skills-dir / --commands-dir flags to keep the embedded
# example skills alongside your own (your entries win on name collisions).
CMD ["--skills-dir", "/skills", "--commands-dir", "/commands", "--transport", "http", "--addr", ":8080", "--log-level", "debug"]
