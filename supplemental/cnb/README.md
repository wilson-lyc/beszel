# CNB container builds

Pushing `main` to the `cnb` remote triggers `.cnb.yml`. The pipeline builds the web UI, then publishes six Linux amd64 image variants using the existing Dockerfiles. CNB's Docker service supplies registry authentication; no access token is committed.

For `wilson-lyc/beszel`, the images are:

| Variant | Image |
| --- | --- |
| Hub | `docker.cnb.cool/wilson-lyc/beszel:latest` |
| Agent | `docker.cnb.cool/wilson-lyc/beszel/beszel-agent:latest` |
| Alpine agent | `docker.cnb.cool/wilson-lyc/beszel/beszel-agent:alpine` |
| NVIDIA agent | `docker.cnb.cool/wilson-lyc/beszel/beszel-agent-nvidia:latest` |
| NVIDIA slim agent | `docker.cnb.cool/wilson-lyc/beszel/beszel-agent-nvidia:slim` |
| Intel agent | `docker.cnb.cool/wilson-lyc/beszel/beszel-agent-intel:latest` |

Each build also publishes `sha-<full-commit-sha>`; Alpine and NVIDIA slim use `sha-<full-commit-sha>-alpine` and `sha-<full-commit-sha>-slim`, respectively. Use these tags to deploy the same revision on Hub and Agent and to roll back.

## Deploying

Replace the `image:` value in the existing Compose configuration, preserving data volumes, KEY, TOKEN, HUB_URL, and other settings. On each deployment host, run:

```sh
docker compose pull
docker compose up -d
```

Private artifacts require `docker login docker.cnb.cool -u cnb` using a CNB access token with artifact read access as the password.

For NVIDIA process monitoring, install/configure NVIDIA Container Toolkit on the Linux host and grant the Agent GPU access (`capabilities: [gpu]`) with `NVIDIA_DRIVER_CAPABILITIES=compute,utility`. Share the host PID namespace (`pid: host`) so the Agent can read the PIDs reported by NVIDIA. Usernames are resolved using the Agent's user database; host accounts must be resolvable inside the container. Keep the runtime as root or supply the permissions required to read process metadata. The slim image additionally needs the host `nvidia-smi` binary mounted as documented in its Dockerfile.

## References

- https://docs.cnb.cool/zh/build/grammar.html
- https://docs.cnb.cool/zh/artifact/docker.html
- https://docs.cnb.cool/zh/guide/git-access.html
