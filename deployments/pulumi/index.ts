import * as awsx from "@pulumi/awsx";
import * as k8s from "@pulumi/kubernetes";
import * as kx from "@pulumi/kubernetesx";
import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const name = pulumi.getProject();

const namespace = new k8s.core.v1.Namespace(name, {
  metadata: {
    name,
  },
});

const postgres = new k8s.helm.v3.Chart(`postgres`, {
  namespace: namespace.metadata.name,
  fetchOpts: {
    repo: "https://charts.bitnami.com/bitnami",
  },
  chart: "postgresql",
  version: "10.3.11",
  values: {
    global: { storageClass: "gp2" },
    postgresqlPassword: config.requireSecret("postgresPassword"),
    postgresqlPostgresPassword: config.requireSecret("postgresPassword"),
    rbac: { create: true },
    volumePermissions: { enabled: true },
  },
});

const repository = new awsx.ecr.Repository(name, {});
const image = repository.buildAndPushImage({
  context: "../../",
  dockerfile: "../../Dockerfile",
  cacheFrom: {
    stages: ["build"],
  },
  env: { DOCKER_BUILDKIT: "1" },
});

const pod = new kx.PodBuilder({
  containers: [
    {
      env: {
        CHAT_PG_HOST: "postgres-postgresql",
        CHAT_PG_PASSWORD: config.requireSecret("postgresPassword"),
      },
      image,
      ports: { http: 8080, admin: 8081 },
      resources: {
        requests: { cpu: "200m", memory: "64Mi" },
        limits: { cpu: "200m", memory: "64Mi" },
      },
      readinessProbe: {
        httpGet: { path: "/check", port: "http" },
      },
      livenessProbe: {
        httpGet: { path: "/healthz", port: "admin" },
        initialDelaySeconds: 30,
      },
    },
  ],
});

const deployment = new kx.Deployment(name, {
  metadata: {
    namespace: namespace.metadata.name,
  },
  spec: pod.asDeploymentSpec({
    replicas: 2,
    strategy: { rollingUpdate: { maxUnavailable: 0 } },
  }),
});

const service = deployment.createService();

const pdb = new k8s.policy.v1beta1.PodDisruptionBudget(name, {
  metadata: {
    namespace: deployment.metadata.namespace,
  },
  spec: {
    maxUnavailable: 1,
    selector: deployment.spec.selector,
  },
});

const ingressMiddleware = new k8s.apiextensions.CustomResource(name, {
  apiVersion: "traefik.containo.us/v1alpha1",
  kind: "Middleware",
  metadata: { namespace: deployment.metadata.namespace },
  spec: {
    rateLimit: {
      average: 300,
      burst: 100,
    },
  },
});

const ingressRoute = new k8s.apiextensions.CustomResource(name, {
  apiVersion: "traefik.containo.us/v1alpha1",
  kind: "IngressRoute",
  metadata: {
    namespace: deployment.metadata.namespace,
  },
  spec: {
    entryPoints: ["websecure"],
    routes: [
      {
        match: "Host(`chat.aaronbatilo.dev`) && PathPrefix(`/`)",
        kind: "Rule",
        middlewares: [{ name: ingressMiddleware.metadata.name }],
        services: [
          { name: service.metadata.name, port: service.spec.ports[0].port },
        ],
      },
      {
        match: "Host(`chat.aaronbatilo.dev`) && PathPrefix(`/metrics`)",
        kind: "Rule",
        middlewares: [{ name: ingressMiddleware.metadata.name }],
        services: [
          { name: service.metadata.name, port: service.spec.ports[1].port },
        ],
      },
    ],
  },
});
