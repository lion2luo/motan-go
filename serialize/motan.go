package serialize

import (
	"errors"
	motan "github.com/weibocom/motan-go/core"
	"io"
	"math"
	"reflect"
)

// serialize type
const (
	mstFalse     = 0
	mstTrue      = 1
	mstNull      = 2
	mstByte      = 3
	mstString    = 4
	mstByteArray = 5
	mstInt16     = 6
	mstInt32     = 7
	mstInt64     = 8
	mstFloat32   = 9
	mstFloat64   = 10

	mstUnpackedArray    = 20
	mstUnpackedArrayEnd = 21
	mstUnpackedMap      = 22
	mstUnpackedMapEnd   = 23

	mstPackedArray = 24
	mstPackedMap   = 25

	mstMessage = 26
)

const MotanSerializationDefaultBufferSize = 2048

var (
	MotanSerializationErrNotSupport = errors.New("not support type by SimpleSerialization")
	MotanSerializationErrWrongSize  = errors.New("read byte size not correct")
)

type GenericMessage struct {
	fields map[int]interface{}
}

func NewGenericMessage() *GenericMessage {
	return &GenericMessage{fields: make(map[int]interface{})}
}

func (m *GenericMessage) GetField(field int) interface{} {
	return m.fields[field]
}

func (m *GenericMessage) SetField(field int, v interface{}) {
	m.fields[field] = v
}

func (m GenericMessage) Range(f func(k int, v interface{}) bool) {
	for k, v := range m.fields {
		if !f(k, v) {
			break
		}
	}
}

type MotanSerialization struct {
}

func (s *MotanSerialization) GetSerialNum() int {
	return 8
}

func (s *MotanSerialization) Serialize(v interface{}) ([]byte, error) {
	buf := motan.NewBytesBuffer(MotanSerializationDefaultBufferSize)
	err := motanSerializeBuf(v, buf)
	return buf.Bytes(), err
}

func (s *MotanSerialization) SerializeMulti(v []interface{}) ([]byte, error) {
	if len(v) == 0 {
		return nil, nil
	}
	buf := motan.NewBytesBuffer(MotanSerializationDefaultBufferSize)
	for _, o := range v {
		err := motanSerializeBuf(o, buf)
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func motanSerializeBuf(v interface{}, buf *motan.BytesBuffer) error {
	if v == nil {
		buf.WriteByte(sNull)
		return nil
	}
	var rv reflect.Value
	if nrv, ok := v.(reflect.Value); ok {
		rv = nrv
	} else {
		rv = reflect.ValueOf(v)
	}
	k := rv.Kind()
	if k == reflect.Interface {
		rv = reflect.ValueOf(rv.Interface())
		k = rv.Kind()
	}

	switch k {
	case reflect.String:
		motanEncodeString(rv.String(), buf)
	case reflect.Bool:
		motanEncodeBool(rv.Bool(), buf)
	case reflect.Uint8:
		motanEncodeByte(byte(rv.Uint()), buf)
	case reflect.Int16:
		motanEncodeInt16(rv.Int(), buf)
	case reflect.Int32:
		motanEncodeInt32(rv.Int(), buf)
	case reflect.Int, reflect.Int64:
		motanEncodeInt64(rv.Int(), buf)
	case reflect.Float32:
		motanEncodeFloat32(rv.Float(), buf)
	case reflect.Float64:
		motanEncodeFloat64(rv.Float(), buf)
	case reflect.Slice:
		if rv.Type().String() == "[]uint8" {
			motanEncodeBytes(rv.Bytes(), buf)
		} else {
			return motanEncodeArray(rv, buf)
		}
	case reflect.Map:
		return motanEncodeMap(rv, buf)
	default:
		return MotanSerializationErrNotSupport
	}
	return nil
}

func (s *MotanSerialization) DeSerialize(b []byte, v interface{}) (interface{}, error) {
	if len(b) == 0 {
		return nil, nil
	}
	buf := motan.CreateBytesBuffer(b)
	return motanDeSerializeBuf(buf, v)
}

func (s *MotanSerialization) DeSerializeMulti(b []byte, v []interface{}) (ret []interface{}, err error) {
	ret = make([]interface{}, 0, len(v))
	buf := motan.CreateBytesBuffer(b)
	if v != nil {
		for _, o := range v {
			rv, err := motanDeSerializeBuf(buf, o)
			if err != nil {
				return nil, err
			}
			ret = append(ret, rv)
		}
	} else {
		for buf.Remain() > 0 {
			rv, err := motanDeSerializeBuf(buf, nil)
			if err != nil {
				if err == io.EOF {
					break
				} else {
					return nil, err
				}
			}
			ret = append(ret, rv)
		}
	}

	return ret, nil
}

func motanDeSerializeBuf(buf *motan.BytesBuffer, v interface{}) (interface{}, error) {
	tp, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	switch int(tp) {
	case mstFalse:
		return motanDecodeBool(tp, v)
	case mstTrue:
		return motanDecodeBool(tp, v)
	case mstNull:
		return nil, nil
	case mstByte:
		return motanDecodeByte(buf, v)
	case mstString:
		return motanDecodeString(buf, v)
	case mstByteArray:
		return motanDecodeBytes(buf, v)
	case mstInt16:
		return motanDecodeInt16(buf, v)
	case mstInt32:
		return motanDecodeInt32(buf, v)
	case mstInt64:
		return motanDecodeInt64(buf, v)
	case mstFloat32:
		return motanDecodeFloat32(buf, v)
	case mstFloat64:
		return motanDecodeFloat64(buf, v)
	case mstUnpackedArray:
		return motanDecodeArray(buf, v)
	case mstUnpackedMap:
		return motanDecodeMap(buf, v)
	case mstMessage:
		return motanDecodeMessage(buf, v)
	}
	return nil, MotanSerializationErrNotSupport
}

func motanEncodeString(s string, buf *motan.BytesBuffer) {
	buf.WriteByte(mstString)
	motanEncodeStringNoTag(s, buf)
}

func motanEncodeStringNoTag(s string, buf *motan.BytesBuffer) {
	b := []byte(s)
	l := len(b)
	buf.WriteZigzag32(uint32(l))
	buf.Write(b)
}

func motanEncodeBytes(b []byte, buf *motan.BytesBuffer) {
	buf.WriteByte(mstByteArray)
	buf.WriteZigzag32(uint32(len(b)))
	buf.Write(b)
}

func motanEncodeBool(b bool, buf *motan.BytesBuffer) {
	if b {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
}

func motanEncodeArray(v reflect.Value, buf *motan.BytesBuffer) error {
	buf.WriteByte(mstUnpackedArray)
	var err error
	for i := 0; i < v.Len(); i++ {
		err = motanSerializeBuf(v.Index(i), buf)
		if err != nil {
			return err
		}
	}
	buf.WriteByte(mstUnpackedArrayEnd)
	return nil
}

func motanEncodeMap(v reflect.Value, buf *motan.BytesBuffer) error {
	buf.WriteByte(mstUnpackedMap)
	var err error
	for _, mk := range v.MapKeys() {
		err = motanSerializeBuf(mk, buf)
		if err != nil {
			return err
		}
		err = motanSerializeBuf(v.MapIndex(mk), buf)
		if err != nil {
			return err
		}
	}
	buf.WriteByte(mstUnpackedMapEnd)
	return err
}

func motanEncodeByte(i byte, buf *motan.BytesBuffer) {
	buf.WriteByte(mstByte)
	buf.WriteByte(i)
}

func motanEncodeInt16(i int64, buf *motan.BytesBuffer) {
	buf.WriteByte(mstInt16)
	buf.WriteUint16(uint16(i))
}

func motanEncodeInt32(i int64, buf *motan.BytesBuffer) {
	buf.WriteByte(mstInt32)
	buf.WriteZigzag32(uint32(i))
}

func motanEncodeInt64(i int64, buf *motan.BytesBuffer) {
	buf.WriteByte(mstInt64)
	buf.WriteZigzag64(uint64(i))
}

func motanEncodeFloat32(f float64, buf *motan.BytesBuffer) {
	buf.WriteByte(mstFloat32)
	buf.WriteUint32(math.Float32bits(float32(f)))
}

func motanEncodeFloat64(f float64, buf *motan.BytesBuffer) {
	buf.WriteByte(mstFloat64)
	buf.WriteUint64(math.Float64bits(f))
}

func motanEncodeMessage(v interface{}, buffer *motan.BytesBuffer) {

}

func motanDecodeBool(b byte, v interface{}) (bool, error) {
	var ret bool
	if b == 1 {
		ret = true
	}
	if v != nil {
		if sv, ok := v.(*bool); ok {
			*sv = ret
		}
	}
	return ret, nil
}

func motanDecodeString(buf *motan.BytesBuffer, v interface{}) (string, error) {
	size, err := buf.ReadZigzag32()
	if err != nil {
		return "", err
	}
	b, err := buf.Next(int(size))
	if err != nil {
		return "", motan.ErrNotEnough
	}
	if v != nil {
		if sv, ok := v.(*string); ok {
			*sv = string(b)
			return *sv, nil
		}
	}
	return string(b), nil
}

func motanDecodeBytes(buf *motan.BytesBuffer, v interface{}) ([]byte, error) {
	size, err := buf.ReadZigzag32()
	if err != nil {
		return nil, err
	}
	b, err := buf.Next(int(size))
	if err != nil {
		return nil, motan.ErrNotEnough
	}
	if v != nil {
		if bv, ok := v.(*[]byte); ok {
			*bv = b
		}
	}
	return b, nil
}

func motanDecodeMap(buf *motan.BytesBuffer, v interface{}) (map[interface{}]interface{}, error) {
	m := make(map[interface{}]interface{}, 32)
	var k interface{}
	var tv interface{}
	for {
		t, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		if t == mstUnpackedMapEnd {
			break
		}
		buf.SetRPos(buf.GetRPos() - 1)
		k, err = motanDeSerializeBuf(buf, nil)
		if err != nil {
			return nil, err
		}
		tv, err = motanDeSerializeBuf(buf, nil)
		if err != nil {
			return nil, err
		}
		m[k] = tv
	}
	if v != nil {
		if rv, ok := v.(*map[interface{}]interface{}); ok {
			*rv = m
		}
	}
	return m, nil
}

func motanDecodeArray(buf *motan.BytesBuffer, v interface{}) ([]interface{}, error) {
	a := make([]interface{}, 0, 32)
	var tv interface{}
	for {
		t, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		if t == mstUnpackedArrayEnd {
			break
		}
		buf.SetRPos(buf.GetRPos() - 1)
		tv, err = motanDeSerializeBuf(buf, nil)
		if err != nil {
			return nil, err
		}
		a = append(a, tv)
	}
	if v != nil {
		if rv, ok := v.(*[]interface{}); ok {
			*rv = a
		}
	}
	return a, nil
}

func motanDecodeByte(buf *motan.BytesBuffer, v interface{}) (byte, error) {
	b, err := buf.ReadByte()
	if err != nil {
		return 0, err
	}
	if v != nil {
		if bv, ok := v.(*byte); ok {
			*bv = byte(b)
		}
	}
	return byte(b), nil
}

func motanDecodeInt16(buf *motan.BytesBuffer, v interface{}) (int16, error) {
	i, err := buf.ReadUint16()
	if err != nil {
		return 0, err
	}
	if v != nil {
		if bv, ok := v.(*int16); ok {
			*bv = int16(i)
		}
	}
	return int16(i), nil
}

func motanDecodeInt32(buf *motan.BytesBuffer, v interface{}) (int32, error) {
	i, err := buf.ReadZigzag32()
	if err != nil {
		return 0, err
	}
	if v != nil {
		if bv, ok := v.(*int32); ok {
			*bv = int32(i)
		}
	}
	return int32(i), nil
}

func motanDecodeInt64(buf *motan.BytesBuffer, v interface{}) (int64, error) {
	i, err := buf.ReadZigzag64()
	if err != nil {
		return 0, err
	}
	if v != nil {
		if bv, ok := v.(*int64); ok {
			*bv = int64(i)
		}
		if bv, ok := v.(*int); ok {
			*bv = int(i)
		}
	}
	return int64(i), nil
}

func motanDecodeFloat32(buf *motan.BytesBuffer, v interface{}) (float32, error) {
	i, err := buf.ReadUint32()
	if err != nil {
		return 0, err
	}
	f := math.Float32frombits(i)
	if v != nil {
		if bv, ok := v.(*float32); ok {
			*bv = f
		}
	}
	return f, nil
}

func motanDecodeFloat64(buf *motan.BytesBuffer, v interface{}) (float64, error) {
	i, err := buf.ReadUint64()
	if err != nil {
		return 0, err
	}
	f := math.Float64frombits(i)
	if v != nil {
		if bv, ok := v.(*float64); ok {
			*bv = f
		}
	}
	return f, nil
}

func motanDecodeMessage(buf *motan.BytesBuffer, i interface{}) (interface{}, error) {
	return nil, nil
}
