package serialize

import (
	motan "github.com/weibocom/motan-go/core"
	"github.com/weibocom/motan-go/serialize/mesh"
)

const (
	Simple = "simple"
	Pb     = "protobuf"
	GrpcPb = "grpc-pb"
	Mesh   = "mesh"
)

func RegistDefaultSerializations(extFactory motan.ExtensionFactory) {
	extFactory.RegistryExtSerialization(Simple, 6, func() motan.Serialization {
		return &SimpleSerialization{}
	})
	extFactory.RegistryExtSerialization(Pb, 5, func() motan.Serialization {
		return &PbSerialization{}
	})
	extFactory.RegistryExtSerialization(GrpcPb, 1, func() motan.Serialization {
		return &GrpcPbSerialization{}
	})
	extFactory.RegistryExtSerialization(Mesh, 1, func() motan.Serialization {
		return &mesh.Serialization{}
	})
}
