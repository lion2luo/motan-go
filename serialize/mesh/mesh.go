package mesh

import (
	"errors"
	"io"
	"math"
	"reflect"
	"strconv"

	motan "github.com/weibocom/motan-go/core"
)

// TypeAlias

// serialize type
const (
	TFalse     = 0
	TTrue      = 1
	TNull      = 2
	TByte      = 3
	TString    = 4
	TByteArray = 5
	TInt16     = 6
	TInt32     = 7
	TInt64     = 8
	TFloat32   = 9
	TFloat64   = 10

	TArray    = 20
	TArrayEnd = 21
	TMap      = 22
	TMapEnd   = 23

	TPackedArray = 24
	TPackedMap   = 25

	TMessage = 26
)

const MotanSerializationDefaultBufferSize = 2048

var (
	ErrTypeUnsupported = errors.New("unsupported type")
	ErrWrongSize       = errors.New("read byte size not correct")
)

type Message interface {
	Marshal(buf *motan.BytesBuffer) error
	Unmarshal(buf *motan.BytesBuffer) error
}

var (
	messageType        = reflect.TypeOf(new(Message)).Elem()
	interfaceSliceType = reflect.TypeOf(make([]interface{}, 0, 0))
	interfaceMapType   = reflect.TypeOf(make(map[interface{}]interface{}))
)

type GenericMessage struct {
	fields map[uint32]interface{}
}

func NewGenericMessage() *GenericMessage {
	return &GenericMessage{fields: make(map[uint32]interface{})}
}

func (m *GenericMessage) GetField(field uint32) interface{} {
	return m.fields[field]
}

func (m *GenericMessage) SetField(field uint32, v interface{}) {
	m.fields[field] = v
}

func (m *GenericMessage) Range(f func(k uint32, v interface{}) bool) {
	for k, v := range m.fields {
		if !f(k, v) {
			break
		}
	}
}

func (m *GenericMessage) Marshal(buf *motan.BytesBuffer) error {
	buf.WriteByte(TMessage)
	pos := buf.GetWPos()
	buf.SetWPos(pos + 4)
	for k, v := range m.fields {
		if v == nil {
			continue
		}
		buf.WriteZigzag32(k)
		if err := SerializeBuf(v, buf); err != nil {
			return err
		}
	}
	nPos := buf.GetWPos()
	buf.SetWPos(pos)
	buf.WriteUint32(uint32(nPos - pos - 4))
	buf.SetWPos(nPos)
	return nil
}

func (m *GenericMessage) Unmarshal(buf *motan.BytesBuffer) error {
	total, err := buf.ReadUint32()
	if err != nil {
		return err
	}
	if total <= 0 {
		return nil
	}
	pos := buf.GetRPos()
	endPos := pos + int(total)
	for buf.GetRPos() < endPos {
		filedNumber, err := buf.ReadZigzag32()
		if err != nil {
			return err
		}
		value, err := DeSerializeBuf(buf, nil)
		if err != nil {
			return err
		}
		m.SetField(uint32(filedNumber), value)
	}
	if buf.GetRPos() != endPos {
		return ErrWrongSize
	}
	return nil
}

type Serialization struct {
}

func (s *Serialization) GetSerialNum() int {
	return 8
}

func (s *Serialization) Serialize(v interface{}) ([]byte, error) {
	buf := motan.NewBytesBuffer(MotanSerializationDefaultBufferSize)
	err := SerializeBuf(v, buf)
	return buf.Bytes(), err
}

func (s *Serialization) SerializeMulti(v []interface{}) ([]byte, error) {
	if len(v) == 0 {
		return nil, nil
	}
	buf := motan.NewBytesBuffer(MotanSerializationDefaultBufferSize)
	for _, o := range v {
		err := SerializeBuf(o, buf)
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func SerializeBuf(v interface{}, buf *motan.BytesBuffer) error {
	if v == nil {
		buf.WriteByte(TNull)
		return nil
	}

	// For struct instance
	if msg, ok := v.(Message); ok {
		return msg.Marshal(buf)
	}

	// For builtin types
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
		EncodeString(rv.String(), buf)
	case reflect.Bool:
		EncodeBool(rv.Bool(), buf)
	case reflect.Uint8:
		EncodeByte(byte(rv.Uint()), buf)
	case reflect.Int16:
		EncodeInt16(int16(rv.Int()), buf)
	case reflect.Int32:
		EncodeInt32(int32(rv.Int()), buf)
	case reflect.Int, reflect.Int64:
		EncodeInt64(rv.Int(), buf)
	case reflect.Float32:
		EncodeFloat32(float32(rv.Float()), buf)
	case reflect.Float64:
		EncodeFloat64(rv.Float(), buf)
	case reflect.Slice:
		if rv.Type().String() == "[]uint8" {
			EncodeBytes(rv.Bytes(), buf)
		} else {
			return EncodeArray(rv, buf)
		}
	case reflect.Map:
		return EncodeMap(rv, buf)
	default:
		return ErrTypeUnsupported
	}
	return nil
}

func (s *Serialization) DeSerialize(b []byte, v interface{}) (interface{}, error) {
	if len(b) == 0 {
		return nil, nil
	}
	buf := motan.CreateBytesBuffer(b)
	return DeSerializeBuf(buf, v)
}

func (s *Serialization) DeSerializeMulti(b []byte, v []interface{}) (ret []interface{}, err error) {
	ret = make([]interface{}, 0, len(v))
	buf := motan.CreateBytesBuffer(b)
	if v != nil {
		for _, o := range v {
			rv, err := DeSerializeBuf(buf, o)
			if err != nil {
				return nil, err
			}
			ret = append(ret, rv)
		}
	} else {
		for buf.Remain() > 0 {
			rv, err := DeSerializeBuf(buf, nil)
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

func DeSerializeBuf(buf *motan.BytesBuffer, v interface{}) (result interface{}, err error) {
	tag, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	rv, ok := v.(*interface{})
	if ok {
		v = nil
	}
	if tag == TNull {
		return nil, nil
	}
	buf.SetRPos(buf.GetRPos() - 1)
	switch int(tag) {
	case TFalse:
		result, err = DecodeBool(buf, v)
	case TTrue:
		result, err = DecodeBool(buf, v)
	case TByte:
		result, err = DecodeByte(buf, v)
	case TString:
		result, err = DecodeString(buf, v)
	case TByteArray:
		result, err = DecodeBytes(buf, v)
	case TInt16:
		result, err = DecodeInt16(buf, v)
	case TInt32:
		result, err = DecodeInt32(buf, v)
	case TInt64:
		result, err = DecodeInt64(buf, v)
	case TFloat32:
		result, err = DecodeFloat32(buf, v)
	case TFloat64:
		result, err = DecodeFloat64(buf, v)
	case TArray:
		result, err = DecodeArray(buf, v)
	case TMap:
		result, err = DecodeMap(buf, v)
	case TMessage:
		if msg, ok := v.(Message); ok {
			err = msg.Unmarshal(buf)
			if err != nil {
				return nil, err
			}
			result = v
			break
		}
		if v == nil {
			msg := NewGenericMessage()
			err = msg.Unmarshal(buf)
			if err != nil {
				return nil, err
			}
			result = v
			break
		}
		if tv, ok := v.(reflect.Type); ok && tv.Implements(messageType) {
			msg := reflect.New(tv)
			err = msg.Interface().(Message).Unmarshal(buf)
			if err != nil {
				return nil, err
			}
			result = msg.Interface()
			break
		}
		return nil, ErrTypeUnsupported
	default:
		return nil, ErrTypeUnsupported
	}
	if rv != nil {
		*rv = result
	}
	return result, err
}

func EncodeString(str string, buf *motan.BytesBuffer) {
	buf.WriteByte(TString)
	EncodeStringNoTag(str, buf)
}

func EncodeStringNoTag(str string, buf *motan.BytesBuffer) {
	b := []byte(str)
	l := len(b)
	buf.WriteZigzag32(uint32(l))
	buf.Write(b)
}

func EncodeBytes(b []byte, buf *motan.BytesBuffer) {
	buf.WriteByte(TByteArray)
	buf.WriteZigzag32(uint32(len(b)))
	buf.Write(b)
}

func EncodeBool(b bool, buf *motan.BytesBuffer) {
	if b {
		buf.WriteByte(TTrue)
	} else {
		buf.WriteByte(TFalse)
	}
}

func EncodeArray(v reflect.Value, buf *motan.BytesBuffer) error {
	buf.WriteByte(TArray)
	var err error
	for i := 0; i < v.Len(); i++ {
		err = SerializeBuf(v.Index(i), buf)
		if err != nil {
			return err
		}
	}
	buf.WriteByte(TArrayEnd)
	return nil
}

func EncodeMap(v reflect.Value, buf *motan.BytesBuffer) error {
	buf.WriteByte(TMap)
	var err error
	for _, mk := range v.MapKeys() {
		err = SerializeBuf(mk, buf)
		if err != nil {
			return err
		}
		err = SerializeBuf(v.MapIndex(mk), buf)
		if err != nil {
			return err
		}
	}
	buf.WriteByte(TMapEnd)
	return err
}

func EncodeByte(i byte, buf *motan.BytesBuffer) {
	buf.WriteByte(TByte)
	buf.WriteByte(i)
}

func EncodeInt16(i int16, buf *motan.BytesBuffer) {
	buf.WriteByte(TInt16)
	buf.WriteUint16(uint16(i))
}

func EncodeInt32(i int32, buf *motan.BytesBuffer) {
	buf.WriteByte(TInt32)
	buf.WriteZigzag32(uint32(i))
}

func EncodeInt64(i int64, buf *motan.BytesBuffer) {
	buf.WriteByte(TInt64)
	buf.WriteZigzag64(uint64(i))
}

func EncodeFloat32(f float32, buf *motan.BytesBuffer) {
	buf.WriteByte(TFloat32)
	buf.WriteUint32(math.Float32bits(f))
}

func EncodeFloat64(f float64, buf *motan.BytesBuffer) {
	buf.WriteByte(TFloat64)
	buf.WriteUint64(math.Float64bits(f))
}

func DecodeBool(buf *motan.BytesBuffer, v interface{}) (bool, error) {
	b, err := buf.ReadByte()
	if err != nil {
		return false, err
	}
	var ret bool
	if b == TTrue {
		ret = true
	} else if b == TFalse {
		ret = false
	} else {
		return false, errors.New("byte tag expected, but actual tag is " + strconv.Itoa(int(b)))
	}
	if v != nil {
		if sv, ok := v.(*bool); ok {
			*sv = ret
		}
	}
	return ret, nil
}

func DecodeString(buf *motan.BytesBuffer, v interface{}) (string, error) {
	tag, err := buf.ReadByte()
	if err != nil {
		return "", err
	}
	if tag == TNull {
		return "", nil
	}
	if tag != TString {
		return "", errors.New("string tag expected, but actual tag is " + strconv.Itoa(int(tag)))
	}
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

func DecodeBytes(buf *motan.BytesBuffer, v interface{}) ([]byte, error) {
	tag, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	if tag == TNull {
		return nil, nil
	}
	if tag != TByteArray {
		return nil, errors.New("byteArray tag expected, but actual tag is " + strconv.Itoa(int(tag)))
	}

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

func DecodeMap(buf *motan.BytesBuffer, v interface{}) (interface{}, error) {
	tag, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	if tag == TNull {
		return nil, nil
	}
	if tag != TMap {
		return nil, errors.New("map tag expected, but actual tag is " + strconv.Itoa(int(tag)))
	}

	var typeOfV reflect.Type
	var pointerV reflect.Value
	if _, ok := v.(reflect.Type); !ok {
		t := reflect.ValueOf(v)
		if t.Kind() == reflect.Ptr {
			pointerV = t.Elem()
			typeOfV = pointerV.Type()
		} else if v == nil {
			typeOfV = interfaceMapType
		} else {
			typeOfV = t.Type()
		}
	} else {
		typeOfV = v.(reflect.Type)
	}

	if typeOfV.Kind() == reflect.Map {
		resultMap := reflect.MakeMapWithSize(typeOfV, 16)
		keyType := typeOfV.Key()
		valueType := typeOfV.Elem()
		for {
			t, err := buf.ReadByte()
			if err != nil {
				return nil, err
			}
			if t == TMapEnd {
				break
			}
			buf.SetRPos(buf.GetRPos() - 1)
			key := reflect.New(keyType)
			_, err = DeSerializeBuf(buf, key.Interface())
			if err != nil {
				return nil, err
			}
			value := reflect.New(valueType)
			_, err = DeSerializeBuf(buf, value.Interface())
			if err != nil {
				return nil, err
			}
			resultMap.SetMapIndex(key.Elem(), value.Elem())
		}
		if pointerV.IsValid() {
			pointerV.Set(resultMap)
		}
		return resultMap.Interface(), nil
	}
	return nil, ErrTypeUnsupported
}

func DecodeArray(buf *motan.BytesBuffer, v interface{}) (interface{}, error) {
	tag, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	if tag == TNull {
		return nil, nil
	}
	if tag != TArray {
		return nil, errors.New("array tag expected, but actual tag is " + strconv.Itoa(int(tag)))
	}

	var typeOfV reflect.Type
	var pointerV reflect.Value
	if _, ok := v.(reflect.Type); !ok {
		t := reflect.ValueOf(v)
		if t.Kind() == reflect.Ptr {
			pointerV = t.Elem()
			typeOfV = pointerV.Type()
		} else if v == nil {
			typeOfV = interfaceSliceType
		} else {
			typeOfV = t.Type()
		}
	} else {
		typeOfV = v.(reflect.Type)
	}

	if typeOfV.Kind() == reflect.Slice {
		resultSlice := reflect.MakeSlice(typeOfV, 0, 32)
		for {
			t, err := buf.ReadByte()
			if err != nil {
				return nil, err
			}
			if t == TArrayEnd {
				break
			}
			buf.SetRPos(buf.GetRPos() - 1)
			value := reflect.New(typeOfV.Elem())
			_, err = DeSerializeBuf(buf, value.Interface())
			if err != nil {
				return nil, err
			}
			resultSlice = reflect.Append(resultSlice, value.Elem())
		}
		if pointerV.IsValid() {
			pointerV.Set(resultSlice)
		}
		return resultSlice.Interface(), nil
	}
	return nil, ErrTypeUnsupported
}

func decodeInteger(buf *motan.BytesBuffer, v interface{}) (uint64, error) {
	var u64 uint64
	var err error
	tag, err := buf.ReadByte()
	if err != nil {
		return 0, err
	}

	if tag == TByte {
		b, err := buf.ReadByte()
		if err != nil {
			return 0, err
		}
		u64 = uint64(b)
	} else if tag == TInt16 {
		u16, err := buf.ReadUint16()
		if err != nil {
			return 0, err
		}
		u64 = uint64(u16)
	} else if tag == TInt32 {
		u64, err = buf.ReadZigzag32()
		if err != nil {
			return 0, err
		}
	} else if tag == TInt64 {
		u64, err = buf.ReadZigzag64()
		if err != nil {
			return 0, err
		}
	} else {
		return 0, errors.New("byte|int16|int32|int64 tag expected, but actual tag is " + strconv.Itoa(int(tag)))
	}

	if v != nil {
		if bv, ok := v.(*byte); ok {
			*bv = byte(u64)
		}
		if bv, ok := v.(*int16); ok {
			*bv = int16(u64)
		}
		if bv, ok := v.(*int32); ok {
			*bv = int32(u64)
		}
		if bv, ok := v.(*int64); ok {
			*bv = int64(u64)
		}
	}
	return u64, nil
}

func DecodeByte(buf *motan.BytesBuffer, v interface{}) (byte, error) {
	integer, err := decodeInteger(buf, v)
	if err != nil {
		return 0, err
	}
	return byte(integer), nil
}

func DecodeInt16(buf *motan.BytesBuffer, v interface{}) (int16, error) {
	integer, err := decodeInteger(buf, v)
	if err != nil {
		return 0, err
	}
	return int16(integer), nil
}

func DecodeInt32(buf *motan.BytesBuffer, v interface{}) (int32, error) {
	integer, err := decodeInteger(buf, v)
	if err != nil {
		return 0, err
	}
	return int32(integer), nil
}

func DecodeInt64(buf *motan.BytesBuffer, v interface{}) (int64, error) {
	integer, err := decodeInteger(buf, v)
	if err != nil {
		return 0, err
	}
	return int64(integer), nil
}

func DecodeFloat32(buf *motan.BytesBuffer, v interface{}) (float32, error) {
	tag, err := buf.ReadByte()
	if err != nil {
		return 0, err
	}
	if tag != TFloat32 {
		return 0, errors.New("float32 tag expected, but actual tag is " + strconv.Itoa(int(tag)))
	}
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

func DecodeFloat64(buf *motan.BytesBuffer, v interface{}) (float64, error) {
	tag, err := buf.ReadByte()
	if err != nil {
		return 0, err
	}
	if tag != TFloat64 {
		return 0, errors.New("float64 tag expected, but actual tag is " + strconv.Itoa(int(tag)))
	}
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
