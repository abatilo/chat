allow_k8s_contexts("arn:aws:eks:us-west-2:717012417639:cluster/usw2-r")
default_registry("ghcr.io/abatilo")

# Allow for installing other helm charts
load("ext://helm_remote", "helm_remote")

# Restart golang process without needing to install a process watcher
load("ext://restart_process", "docker_build_with_restart")

helm_remote("postgresql",
  repo_name="bitnami",
  repo_url="https://charts.bitnami.com/bitnami",
  # This chart version pulls in app version 11.11.0
  version="10.3.11",
  set=[
    "postgresqlPassword=localdev",
    "postgresqlPostgresPassword=localdev",
    "rbac.create=true",
    "volumePermissions.enabled=true"
  ]
)

# When running locally through Tilt, we want to run in dev mode
docker_build_with_restart(
  ref="chat",
  context=".", # From location of Tiltfile
  dockerfile="./Dockerfile",
  live_update=[
    # From location of Tiltfile
    sync("./", "/app/"),

    # Ran within container
    run("cd /app/ && go mod download", trigger=["./go.sum"]),
  ],

  # Override Dockerfile so that we stay on the build layer with dev
  # dependencies and hot reloading
  target="build",
  entrypoint="go run cmd/chat.go api run",
)

k8s_yaml("./deployments/api.yaml")
k8s_resource("api", port_forwards=["8080", "8081"])

k8s_resource("postgresql-postgresql", port_forwards=["5432"])
