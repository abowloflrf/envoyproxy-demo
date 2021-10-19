package main

import (
	"time"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	tlsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes"
)

const (
	RouteName    = "local_route"
	ListenerName = "listener_1"
	ListenerPort = 20000 // 20000 动态 listener

	// 其中一条规则
	ClusterName  = "example_proxy_cluster"
	UpstreamHost = "www.envoyproxy.io"
	UpstreamPort = 443
)

// makeCluster envoy cluster
func makeCluster(clusterName string) *cluster.Cluster {
	tlstransport := &tlsv3.UpstreamTlsContext{
		Sni: UpstreamHost,
	}
	pbst, err := ptypes.MarshalAny(tlstransport)
	if err != nil {
		panic(err)
	}
	return &cluster.Cluster{
		Name:                 clusterName,
		ConnectTimeout:       ptypes.DurationProto(5 * time.Second),
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_LOGICAL_DNS},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment:       makeEndpoint(clusterName),
		DnsLookupFamily:      cluster.Cluster_V4_ONLY,
		TransportSocket: &core.TransportSocket{
			Name: "envoy.transport_sockets.tls",
			ConfigType: &core.TransportSocket_TypedConfig{
				TypedConfig: pbst,
			},
		},
	}
}

// makeEndpoint envoy ep
func makeEndpoint(clusterName string) *endpoint.ClusterLoadAssignment {
	return &endpoint.ClusterLoadAssignment{
		ClusterName: clusterName,
		Endpoints: []*endpoint.LocalityLbEndpoints{{
			LbEndpoints: []*endpoint.LbEndpoint{{
				HostIdentifier: &endpoint.LbEndpoint_Endpoint{
					Endpoint: &endpoint.Endpoint{
						Address: &core.Address{
							Address: &core.Address_SocketAddress{
								SocketAddress: &core.SocketAddress{
									Protocol: core.SocketAddress_TCP,
									Address:  UpstreamHost,
									PortSpecifier: &core.SocketAddress_PortValue{
										PortValue: UpstreamPort,
									},
								},
							},
						},
					},
				},
			}},
		}},
	}
}

// makeDynamicCluster 自定义一个 cluster
func makeDynamicCluster(name string, addrs []string, port uint32) *cluster.Cluster {
	eps := []*endpoint.LbEndpoint{}
	for i := range addrs {
		eps = append(eps, &endpoint.LbEndpoint{
			HostIdentifier: &endpoint.LbEndpoint_Endpoint{
				Endpoint: &endpoint.Endpoint{
					Address: &core.Address{
						Address: &core.Address_SocketAddress{
							SocketAddress: &core.SocketAddress{
								Protocol: core.SocketAddress_TCP,
								Address:  addrs[i],
								PortSpecifier: &core.SocketAddress_PortValue{
									PortValue: port,
								},
							},
						},
					},
				},
			},
		})
	}
	return &cluster.Cluster{
		Name:                 name,
		ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_STATIC},
		LbPolicy:             cluster.Cluster_ROUND_ROBIN,
		LoadAssignment: &endpoint.ClusterLoadAssignment{
			ClusterName: name,
			Endpoints: []*endpoint.LocalityLbEndpoints{{
				LbEndpoints: eps,
			}},
		},
	}
}

// makeRoute 一条 route，多个 VirtualHosts 从上至下匹配，匹配到第一个即可
func makeRoute(routeName string, clusterName string) *route.RouteConfiguration {
	return &route.RouteConfiguration{
		Name: routeName,
		VirtualHosts: []*route.VirtualHost{
			{
				Name:    "demo_service",
				Domains: []string{"demo.ruofeng.me"},
				Routes: []*route.Route{{
					Match: &route.RouteMatch{
						PathSpecifier: &route.RouteMatch_Prefix{
							Prefix: "/",
						},
					},
					Action: &route.Route_Route{
						Route: &route.RouteAction{
							ClusterSpecifier: &route.RouteAction_Cluster{
								Cluster: "local_demo",
							},
						},
					},
				}},
			},
			{
				Name:    "envoy_service",
				Domains: []string{"envoy.ruofeng.me"},
				Routes: []*route.Route{{
					Match: &route.RouteMatch{
						PathSpecifier: &route.RouteMatch_Prefix{
							Prefix: "/",
						},
					},
					Action: &route.Route_Route{
						Route: &route.RouteAction{
							ClusterSpecifier: &route.RouteAction_Cluster{
								Cluster: clusterName,
							},
							HostRewriteSpecifier: &route.RouteAction_HostRewriteLiteral{
								HostRewriteLiteral: UpstreamHost,
							},
						},
					},
				}},
			},
			{
				Name:    "prom_service",
				Domains: []string{"prom.ruofeng.me"},
				Routes: []*route.Route{{
					Match: &route.RouteMatch{
						PathSpecifier: &route.RouteMatch_Prefix{
							Prefix: "/",
						},
					},
					Action: &route.Route_Route{
						Route: &route.RouteAction{
							ClusterSpecifier: &route.RouteAction_Cluster{
								Cluster: "prom_cluster",
							},
						},
					},
				}},
			},
		},
	}
}

func makeHTTPListener(listenerName string, route string) *listener.Listener {
	// HTTP filter configuration
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: "http",
		RouteSpecifier: &hcm.HttpConnectionManager_Rds{
			Rds: &hcm.Rds{
				ConfigSource:    makeConfigSource(),
				RouteConfigName: route,
			},
		},
		HttpFilters: []*hcm.HttpFilter{{
			Name: wellknown.Router,
		}},
	}
	pbst, err := ptypes.MarshalAny(manager)
	if err != nil {
		panic(err)
	}

	return &listener.Listener{
		Name: listenerName,
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: ListenerPort,
					},
				},
			},
		},
		FilterChains: []*listener.FilterChain{{
			Filters: []*listener.Filter{{
				Name: wellknown.HTTPConnectionManager,
				ConfigType: &listener.Filter_TypedConfig{
					TypedConfig: pbst,
				},
			}},
		}},
	}
}

func makeConfigSource() *core.ConfigSource {
	source := &core.ConfigSource{}
	source.ResourceApiVersion = resource.DefaultAPIVersion
	source.ConfigSourceSpecifier = &core.ConfigSource_ApiConfigSource{
		ApiConfigSource: &core.ApiConfigSource{
			TransportApiVersion:       resource.DefaultAPIVersion,
			ApiType:                   core.ApiConfigSource_GRPC,
			SetNodeOnFirstMessageOnly: true,
			GrpcServices: []*core.GrpcService{{
				TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &core.GrpcService_EnvoyGrpc{ClusterName: "xds_cluster"},
				},
			}},
		},
	}
	return source
}

func GenerateSnapshot() cache.Snapshot {

	snap := cache.NewSnapshot("3",
		// &endpoint.ClusterLoadAssignment{
		// 	ClusterName: "node_exporter",
		// 	NamedEndpoints: map[string]*endpoint.Endpoint{
		// 		"dynamic_demo": &endpoint.Endpoint{
		// 			Address: &core.Address{
		// 				Address: &core.Address_SocketAddress{
		// 					SocketAddress: &core.SocketAddress{
		// 						Address: "127.0.0.1",
		// 						PortSpecifier: &core.SocketAddress_PortValue{
		// 							PortValue: 9100,
		// 						},
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// }
		[]types.Resource{},
		[]types.Resource{makeCluster(ClusterName), makeDynamicCluster("prom_cluster", []string{"127.0.0.1"}, 9100)},
		[]types.Resource{makeRoute(RouteName, ClusterName)},
		[]types.Resource{makeHTTPListener(ListenerName, RouteName)},
		[]types.Resource{}, // runtime
		[]types.Resource{}, // secret
	)
	return snap
}
