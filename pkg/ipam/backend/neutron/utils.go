package neutron

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"github.com/yaoice/meliodas/pkg/ipam/backend"
	"github.com/yaoice/meliodas/pkg/ipam/backend/allocator"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/attachinterfaces"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"log"
	"os"
	"strings"
)

var (
	keyText  = "aabcice12798akljzmknm.ahkjkljl;k"
	commonIV = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
)

func ConnectStore(openstackConf *allocator.OpenStackConf, serviceType string) (*gophercloud.ServiceClient, error) {
	return connectWithKeyStone(openstackConf, serviceType)
}

// Keystone v2/v3
func connectWithKeyStone(openstackConf *allocator.OpenStackConf, serviceType string) (*gophercloud.ServiceClient, error) {
	var (
		provider *gophercloud.ProviderClient
		err      error
	)

	if strings.HasSuffix(openstackConf.AuthUrl, "v3") {
		// keystone auth v3
		provider, err = openstack.AuthenticatedClient(gophercloud.AuthOptions{
			IdentityEndpoint: openstackConf.AuthUrl,
			Username:         openstackConf.UserName,
			Password:         encrypt.aesDecrypt(openstackConf.PassWord),
			TenantName:       openstackConf.Project,
			DomainName:       openstackConf.Domain,
		})
	} else {
		// keystone auth v2
		provider, err = openstack.AuthenticatedClient(gophercloud.AuthOptions{
			IdentityEndpoint: openstackConf.AuthUrl,
			Username:         openstackConf.UserName,
			Password:         encrypt.aesDecrypt(openstackConf.PassWord),
			TenantName:       openstackConf.Project,
		})
	}
	if err != nil {
		return nil, err
	}
	// 根据service type类型返回对应的service client
	switch serviceType {
	case backend.SERVICE_TYPE_NETWORK:
		{
			networkClient, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{})
			if err != nil {
				log.Printf("Get network client err: %v", err)
				return nil, err
			}
			return networkClient, err
		}
	case backend.SERVICE_TYPE_COMPUTE:
		{
			computeClient, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{})
			if err != nil {
				log.Printf("Get compute client err: %v", err)
				return nil, err
			}
			return computeClient, err
		}
	default:
		return nil, fmt.Errorf("unsupported keystone service type %s", serviceType)
	}
}

var encrypt = NewEncrypter()

func NewEncrypter() *encrypter {
	// create encrypt algorithm
	cip, err := aes.NewCipher([]byte(keyText))
	if err != nil {
		log.Printf("Get aes cipher err: %v", err)
		return nil
	}
	return &encrypter{
		cip:      cip,
		commonIV: commonIV,
	}
}

type encrypter struct {
	cip      cipher.Block
	commonIV []byte
}

// AES Encrypt
func (e *encrypter) aesEncrypt(plainText string) string {
	plainTextByte := []byte(plainText)
	// encrypt plaintext
	cfb := cipher.NewCFBEncrypter(e.cip, e.commonIV)
	cipherText := make([]byte, len(plainTextByte))
	cfb.XORKeyStream(cipherText, plainTextByte)
	return hex.EncodeToString(cipherText)
}

// AES Decrypt
func (e *encrypter) aesDecrypt(cipherText string) string {
	cipherTextByte, err := hex.DecodeString(cipherText)
	if err != nil {
		panic(err)
	}
	// decrypt cipherText
	cfbdec := cipher.NewCFBDecrypter(e.cip, e.commonIV)
	plaintextCopy := make([]byte, len(cipherTextByte))
	cfbdec.XORKeyStream(plaintextCopy, cipherTextByte)
	return string(plaintextCopy)
}

// get hostname
func getHostName() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// get compute client
func GetComputeClient(ipamConfig *allocator.IPAMConfig) (*gophercloud.ServiceClient, error) {
	computeClient, err := ConnectStore(ipamConfig.OpenStackConf, backend.SERVICE_TYPE_COMPUTE)
	if err != nil {
		return nil, err
	}
	return computeClient, nil
}

// get network client
func GetNetworkClient(ipamConfig *allocator.IPAMConfig) (*gophercloud.ServiceClient, error) {
	networkClient, err := ConnectStore(ipamConfig.OpenStackConf, backend.SERVICE_TYPE_NETWORK)
	if err != nil {
		return nil, err
	}
	return networkClient, nil
}

// find server
func FindServer(computeClient *gophercloud.ServiceClient, serverIP string) (*servers.Server, error) {
	serverList, err := servers.List(computeClient, servers.ListOpts{}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("get server list err: %v", err)
	}
	serverSlice, err := servers.ExtractServers(serverList)
	if err != nil {
		return nil, err
	}
	for _, server := range serverSlice {
		for _, v := range server.Addresses {
			if strings.Contains(fmt.Sprintf("%v", v), serverIP) {
				return &server, nil
			}
		}
	}
	return nil, nil
}

func CountServerIntsAndEnis(computeClient *gophercloud.ServiceClient, serverIP string) (int, int, error) {
	server, err := FindServer(computeClient, serverIP)
	if err != nil {
		return 0, 0, err
	}
	attachInterfaces, err := attachinterfaces.List(computeClient, server.ID).AllPages()
	if err != nil {
		log.Printf("Instance %s list attachinterfaces err: %v", server.ID, err.Error())
		return 0, 0, err
	}
	attachInterfacesSlice, err := attachinterfaces.ExtractInterfaces(attachInterfaces)
	if err != nil {
		return 0, 0, err
	}
	return len(attachInterfacesSlice),len(server.Addresses), nil
}

// find port
func FindPort(networkClient *gophercloud.ServiceClient, portName string) (*ports.Port, error) {
	portList, err := ports.List(networkClient, ports.ListOpts{Name: portName, Limit: 1}).AllPages()
	if err != nil {
		log.Printf("get neutron port name %s err: %v", portName, err.Error())
		return nil, err
	}
	portSlice, err := ports.ExtractPorts(portList)
	if err != nil {
		return nil, err
	}
	if len(portSlice) < 1 {
		return nil, nil
	}
	return &portSlice[0], nil
}

// find router
func FindRouter(client *gophercloud.ServiceClient, gw string) (*routers.Router, error) {
	routerList, err := routers.List(client, routers.ListOpts{}).AllPages()
	if err != nil {
		log.Printf("get neutron router list err: %v", err.Error())
		return nil, err
	}
	routerSlice, err := routers.ExtractRouters(routerList)
	if err != nil {
		return nil, err
	}
	if len(routerSlice) < 1 {
		return nil, nil
	}

	for _, r := range routerSlice {
		portList, err := ports.List(client, ports.ListOpts{DeviceID: r.ID}).AllPages()
		if err != nil {
			log.Printf("get neutron router ports err: %v", err.Error())
			return nil, err
		}
		portSlice, err := ports.ExtractPorts(portList)
		if err != nil {
			return nil, err
		}
		for _, p := range portSlice {
			for _, fixIP := range p.FixedIPs {
				if fixIP.IPAddress == gw {
					return &r, nil
				}
			}
		}
	}
	return nil, nil
}

