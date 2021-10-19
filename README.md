# envoyproxy 控制面 demo

## 几个配置文件

- `envoy.yaml` - envoy 默认静态配置
- `demo.yaml` - 包含 xDS 动态配置

## 几个监听端口

- 127.0.0.1:18000 - 控制面 xDS gRPC 端口
- 127.0.0.1:9901 - envoyproxy admin 端口
- 127.0.0.1:10000 - envoyproxy 静态配置 lisener
- 127.0.0.1:28000 - envoyproxy 控制面下发的一个动态 lisener
