package routerosv7

import (
	"context"

	"github.com/nveeser/routeros7/session"
	"golang.org/x/sync/errgroup"
)

type CompositeResource interface {
	Requests() []session.Requester
}

type SingletonResource interface {
	CompositeResource
	SingletonPath() string
}

type IdResource interface {
	CollectionPath() string
}

type NameResource interface {
	IdResource
	Name() string
}

func Read(ctx context.Context, c *session.Client, result CompositeResource) error {
	g, cctx := errgroup.WithContext(ctx)
	g.SetLimit(10)
	for _, req := range result.Requests() {
		req := req
		g.Go(func() error {
			return c.Print(cctx, req)
		})
	}
	return g.Wait()
}

func ReadAll[T IdResource](ctx context.Context, c *session.Client, result *[]T) error {
	return c.Print(ctx, requestAll(result))
}

func ReadId[T IdResource](ctx context.Context, c *session.Client, result *T, id string) error {
	return c.Print(ctx, requestId(result, id))
}

func ReadName[T NameResource](ctx context.Context, c *session.Client, result *T, name string) error {
	return c.Print(ctx, requestId(result, name))
}

func requestResource(result CompositeResource) []session.Requester {
	return result.Requests()
}

func requestSingle[T SingletonResource](result T) session.Requester {
	return &session.Request[T]{Command: result.SingletonPath(), Result: result}
}

func requests(r session.Requester) []session.Requester {
	return []session.Requester{r}
}

func requestId[T IdResource](result *T, id string) session.Requester {
	var r T
	return &session.Request[*T]{Command: r.CollectionPath() + "/" + id, Result: result}
}

func requestAll[T IdResource](result *[]T) session.Requester {
	var r T
	return &session.Request[*[]T]{Command: r.CollectionPath(), Result: result}
}

// RouterConfig aggregates all state components.
type RouterConfig struct {
	System   SystemConfig   `yaml:"system" json:"system"`
	Network  NetworkConfig  `yaml:"network" json:"network"`
	IP       IPConfig       `yaml:"ip" json:"ip"`
	Firewall FirewallConfig `yaml:"firewall,omitempty" json:"firewall,omitempty"`
	Routing  RoutingConfig  `yaml:"routing,omitempty" json:"routing,omitempty"`
}

var _ CompositeResource = (*RouterConfig)(nil)

func (r *RouterConfig) Requests() []session.Requester {
	return concatAll(
		r.System.Requests(),
		r.Network.Requests(),
		r.IP.Requests(),
		r.Firewall.Requests(),
		r.Routing.Requests())
}

// --- System Domain (Singletons) ---

type SystemConfig struct {
	Identity IdentityConfig `yaml:"identity" json:"identity"`
	Clock    ClockConfig    `yaml:"clock" json:"clock"`
	NTP      NTPConfig      `yaml:"ntp" json:"ntp"`
}

func (r *SystemConfig) Requests() []session.Requester {
	return concat(
		requestSingle(&r.Identity),
		requestSingle(&r.Clock),
		requestSingle(&r.NTP))
}

type IdentityConfig struct {
	Name string `yaml:"name" json:"name"`
}

func (r *IdentityConfig) SingletonPath() string         { return "system/identity" }
func (r *IdentityConfig) Requests() []session.Requester { return requests(requestSingle(r)) }

type ClockConfig struct {
	TimeZoneName       string `yaml:"time_zone_name" json:"time-zone-name"`
	TimeZoneAutodetect bool   `yaml:"time_zone_autodetect" json:"time-zone-autodetect,string"`
}

func (r *ClockConfig) SingletonPath() string         { return "system/clock" }
func (r *ClockConfig) Requests() []session.Requester { return requests(requestSingle(r)) }

type NTPConfig struct {
	Enabled Mbool  `yaml:"enabled" json:"enabled"`
	Mode    string `yaml:"mode,omitempty" json:"mode,omitempty"`
	Servers MList  `yaml:"servers" json:"servers"`
}

func (r *NTPConfig) SingletonPath() string         { return "system/ntp/client" }
func (r *NTPConfig) Requests() []session.Requester { return requests(requestSingle(r)) }

// --- Network & Interface Domain (Collections) ---

type NetworkConfig struct {
	Bridges     []BridgeConfig     `yaml:"bridges,omitempty" json:"bridges,omitempty"`
	BridgePorts []BridgePortConfig `yaml:"bridge_ports,omitempty" json:"bridge_ports,omitempty"`
	BridgeVLANs []BridgeVLANConfig `yaml:"bridge_vlans,omitempty" json:"bridge_vlans,omitempty"`
	VLANs       []VLANConfig       `yaml:"vlans,omitempty" json:"vlans,omitempty"`
	Ethernet    []EthernetConfig   `yaml:"ethernet,omitempty" json:"ethernet,omitempty"`
}

func (r *NetworkConfig) Requests() []session.Requester {
	return concat(
		requestAll(&r.Bridges),
		requestAll(&r.BridgePorts),
		requestAll(&r.BridgeVLANs),
		requestAll(&r.VLANs),
		requestAll(&r.Ethernet))
}

type BridgeConfig struct {
	Id            string `yaml:"-" json:".id,omitempty"`
	Name          string `yaml:"name" json:"name"`
	VlanFiltering Mbool  `yaml:"vlan_filtering" json:"vlan-filtering"`
	PVID          string `yaml:"pvid,omitempty" json:"pvid,omitempty"`
	Comment       string `yaml:"comment,omitempty" json:"comment,omitempty"`
}

func (r BridgeConfig) CollectionPath() string { return "interface/bridge" }

type BridgePortConfig struct {
	Id         string `yaml:"-" json:".id,omitempty"`
	Bridge     string `yaml:"bridge" json:"bridge"`
	Interface  string `yaml:"interface" json:"interface"`
	PVID       string `yaml:"pvid,omitempty" json:"pvid,omitempty"`
	FrameTypes string `yaml:"frame_types,omitempty" json:"frame-types,omitempty"`
	Comment    string `yaml:"comment,omitempty" json:"comment,omitempty"`
}

func (r BridgePortConfig) CollectionPath() string { return "interface/bridge/port" }

type BridgeVLANConfig struct {
	Id       string `yaml:"-" json:".id,omitempty"`
	Bridge   string `yaml:"bridge" json:"bridge"`
	VLANIDs  MList  `yaml:"vlan_ids" json:"vlan-ids"`
	Tagged   MList  `yaml:"tagged,omitempty" json:"tagged,omitempty"`
	Untagged MList  `yaml:"untagged,omitempty" json:"untagged,omitempty"`
}

func (r BridgeVLANConfig) CollectionPath() string { return "interface/bridge/vlan" }

type VLANConfig struct {
	Id        string `yaml:"-" json:".id,omitempty"`
	Name      string `yaml:"name" json:"name"`
	VLANID    string `yaml:"vlan_id" json:"vlan-id"`
	Interface string `yaml:"interface" json:"interface"`
	Comment   string `yaml:"comment,omitempty" json:"comment,omitempty"`
}

func (r VLANConfig) CollectionPath() string { return "interface/vlan" }

type EthernetConfig struct {
	Id      string `yaml:"-" json:".id,omitempty"`
	Name    string `yaml:"name" json:"name"`
	Speed   string `yaml:"speed,omitempty" json:"speed,omitempty"`
	Comment string `yaml:"comment,omitempty" json:"comment,omitempty"`
}

func (r EthernetConfig) CollectionPath() string { return "interface/ethernet" }

// --- IP Domain ---

type IPConfig struct {
	Addresses []AddressConfig `yaml:"addresses,omitempty" json:"addresses,omitempty"`
	DNS       DNSConfig       `yaml:"dns,omitempty" json:"dns,omitempty"`
	Pools     []PoolConfig    `yaml:"pools,omitempty" json:"pools,omitempty"`
	Routes    []RouteConfig   `yaml:"routes,omitempty" json:"routes,omitempty"`
}

func (r *IPConfig) Requests() []session.Requester {
	return concat(
		requestAll(&r.Addresses),
		requestSingle(&r.DNS),
		requestAll(&r.Pools),
		requestAll(&r.Routes))
}

type AddressConfig struct {
	Id        string `yaml:"id" json:".id,omitempty"`
	Address   string `yaml:"address" json:"address"`
	Interface string `yaml:"interface" json:"interface"`
	Comment   string `yaml:"comment,omitempty" json:"comment,omitempty"`
	Disabled  Mbool  `yaml:"disabled,omitempty" json:"disabled,omitempty"`
}

func (r AddressConfig) CollectionPath() string { return "ip/address" }

type DNSConfig struct {
	Servers        MList  `yaml:"servers,omitempty" json:"servers,omitempty"`
	AllowRemoteReq Mbool  `yaml:"allow_remote_requests" json:"allow-remote-requests"`
	CacheMaxTTL    string `yaml:"cache_max_ttl,omitempty" json:"cache-max-ttl,omitempty"`
}

func (r *DNSConfig) SingletonPath() string         { return "ip/dns" }
func (r *DNSConfig) Requests() []session.Requester { return requests(requestSingle(r)) }

type PoolConfig struct {
	Id     string `yaml:"-" json:".id,omitempty"`
	Name   string `yaml:"name" json:"name"`
	Ranges string `yaml:"ranges" json:"ranges"`
}

func (r PoolConfig) CollectionPath() string { return "ip/pool" }

type RouteConfig struct {
	Id         string `yaml:"-" json:".id,omitempty"`
	DstAddress string `yaml:"dst_address" json:"dst-address"`
	Gateway    string `yaml:"gateway" json:"gateway"`
	Distance   MInt   `yaml:"distance,omitempty" json:"distance,omitempty"`
	Comment    string `yaml:"comment,omitempty" json:"comment,omitempty"`
}

func (r RouteConfig) CollectionPath() string { return "ip/route" }

// --- Firewall Domain ---

type FirewallConfig struct {
	FilterRules []FirewallFilterConfig `yaml:"filter,omitempty" json:"filter,omitempty"`
	NATRules    []FirewallNATConfig    `yaml:"nat,omitempty" json:"nat,omitempty"`
}

func (r *FirewallConfig) Requests() []session.Requester {
	return concat(
		requestAll(&r.FilterRules),
		requestAll(&r.NATRules))
}

type FirewallFilterConfig struct {
	Id          string `yaml:"-" json:".id,omitempty"`
	Chain       string `yaml:"chain" json:"chain"`
	Action      string `yaml:"action" json:"action"`
	Protocol    string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
	DstPort     string `yaml:"dst_port,omitempty" json:"dst-port,omitempty"`
	InInterface string `yaml:"in_interface,omitempty" json:"in-interface,omitempty"`
	Comment     string `yaml:"comment,omitempty" json:"comment,omitempty"`
}

func (r FirewallFilterConfig) CollectionPath() string { return "ip/firewall/filter" }

type FirewallNATConfig struct {
	Id           string `yaml:"-" json:".id,omitempty"`
	Chain        string `yaml:"chain" json:"chain"`
	Action       string `yaml:"action" json:"action"`
	OutInterface string `yaml:"out_interface,omitempty" json:"out-interface,omitempty"`
	ToAddresses  string `yaml:"to_addresses,omitempty" json:"to-addresses,omitempty"`
	Comment      string `yaml:"comment,omitempty" json:"comment,omitempty"`
}

func (r FirewallNATConfig) CollectionPath() string { return "ip/firewall/nat" }

// --- Routing Domain ---

type RoutingConfig struct {
	BGPInstances []BGPInstanceConfig `yaml:"bgp_instances,omitempty" json:"bgp_instances,omitempty"`
	BGPPeers     []BGPPeerConfig     `yaml:"bgp_peers,omitempty" json:"bgp_peers,omitempty"`
}

func (r *RoutingConfig) Requests() []session.Requester {
	return concat(
		requestAll(&r.BGPInstances),
		requestAll(&r.BGPPeers))
}

type BGPInstanceConfig struct {
	Id   string `yaml:"-" json:".id,omitempty"`
	Name string `yaml:"name" json:"name"`
	AS   MInt   `yaml:"as" json:"as"`
}

func (r BGPInstanceConfig) CollectionPath() string { return "routing/bgp/template" }

type BGPPeerConfig struct {
	Id         string `yaml:"-" json:".id,omitempty"`
	Name       string `yaml:"name" json:"name"`
	RemoteAddr string `yaml:"remote_address" json:"remote.address"`
	RemoteAS   MInt   `yaml:"remote_as" json:"remote.as"`
}

func (r BGPPeerConfig) CollectionPath() string { return "routing/bgp/connection" }

func concat(s ...session.Requester) (out []session.Requester) {
	for _, ss := range s {
		out = append(out, ss)
	}
	return
}

func concatAll(s ...[]session.Requester) (out []session.Requester) {
	for _, ss := range s {
		out = append(out, ss...)
	}
	return
}
