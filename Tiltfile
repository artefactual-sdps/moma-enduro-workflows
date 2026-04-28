version_settings(constraint=">=0.35.0")
secret_settings(disable_scrub=True)
load('ext://dotenv', 'dotenv')

# Load tilt env file if it exists
dotenv_path = ".tilt.env"
if os.path.exists(dotenv_path):
  dotenv(fn=dotenv_path)

# Configure trigger mode
true = ("true", "1", "yes", "t", "y")
trigger_mode = TRIGGER_MODE_MANUAL
if os.environ.get('TRIGGER_MODE_AUTO', '').lower() in true:
  trigger_mode = TRIGGER_MODE_AUTO

# Docker images
custom_build(
  ref="preprocessing-moma-worker:dev",
  command=["hack/build_docker.sh"],
  deps=["."],
)

# Kubernetes manifests
k8s_yaml(kustomize("hack/kube"))

# Tilt resources
k8s_resource(
  "preprocessing-worker",
  labels=["Preprocessing"],
  trigger_mode=trigger_mode,
)
