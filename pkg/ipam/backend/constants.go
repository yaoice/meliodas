package backend

const (
	IPAM_MODE_SEPARATE string = "separate"
	IPAM_MODE_MIX string = "mix"
	IPAM_MODE_MIX_ROUTE string = "mix-route"
	IPAM_MODE_ENI string = "eni"
	DefaultDataDir string = "/var/lib/cni/networks"

	SERVICE_TYPE_NETWORK string = "network"
	SERVICE_TYPE_COMPUTE string = "compute"
	MAX_ENIS = 16
	MAX_IPS = 20
)
