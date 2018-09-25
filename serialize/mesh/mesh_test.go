package mesh

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/weibocom/motan-go/core"
)

type TestStruct struct {
	F1  string            `mesh:"1"`
	F2  bool              `mesh:"2"`
	F3  byte              `mesh:"3"`
	F4  int16             `mesh:"4"`
	F5  int32             `mesh:"5"`
	F6  int64             `mesh:"6"`
	F7  float32           `mesh:"7"`
	F8  float64           `mesh:"8"`
	F9  []string          `mesh:"9"`
	F10 map[string]string `mesh:"10"`
	F11 []byte            `mesh:"11"`
	F12 *TestStruct       `mesh:"12"`
}

func (m *TestStruct) Marshal(buf *core.BytesBuffer) error {
	buf.WriteByte(TMessage)
	pos := buf.GetWPos()
	buf.SetWPos(pos + 4)

	buf.WriteZigzag32(1)
	EncodeString(m.F1, buf)

	buf.WriteZigzag32(2)
	EncodeBool(m.F2, buf)

	buf.WriteZigzag32(3)
	EncodeByte(m.F3, buf)

	buf.WriteZigzag32(4)
	EncodeInt16(m.F4, buf)

	buf.WriteZigzag32(5)
	EncodeInt32(m.F5, buf)

	buf.WriteZigzag32(6)
	EncodeInt64(m.F6, buf)

	buf.WriteZigzag32(7)
	EncodeFloat32(m.F7, buf)

	buf.WriteZigzag32(8)
	EncodeFloat64(m.F8, buf)

	if m.F9 != nil {
		buf.WriteZigzag32(9)
		if err := SerializeBuf(m.F9, buf); err != nil {
			return err
		}
	}
	if m.F10 != nil {
		buf.WriteZigzag32(10)
		if err := SerializeBuf(m.F10, buf); err != nil {
			return err
		}
	}
	if m.F11 != nil {
		buf.WriteZigzag32(11)
		EncodeBytes(m.F11, buf)
	}
	if m.F12 != nil {
		buf.WriteZigzag32(12)
		if err := SerializeBuf(m.F11, buf); err != nil {
			return err
		}
	}

	nPos := buf.GetWPos()
	buf.SetWPos(pos)
	buf.WriteUint32(uint32(nPos - pos - 4))
	buf.SetWPos(nPos)
	return nil
}

func (m *TestStruct) Unmarshal(buf *core.BytesBuffer) error {
	tag, _ := buf.ReadByte()
	if tag != TMessage {
		return errors.New("message tag expected, but actual tag is " + strconv.Itoa(int(tag)))
	}
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
		switch filedNumber {
		case 1:
			_, err = DecodeString(buf, &m.F1)
			if err != nil {
				return err
			}
		case 2:
			_, err = DecodeBool(buf, &m.F2)
			if err != nil {
				return err
			}
		case 3:
			_, err = DecodeByte(buf, &m.F3)
			if err != nil {
				return err
			}
		case 4:
			_, err = DecodeInt16(buf, &m.F4)
			if err != nil {
				return err
			}
		case 5:
			_, err = DecodeInt32(buf, &m.F5)
			if err != nil {
				return err
			}
		case 6:
			_, err = DecodeInt64(buf, &m.F6)
			if err != nil {
				return err
			}
		case 7:
			_, err = DecodeFloat32(buf, &m.F7)
			if err != nil {
				return err
			}
		case 8:
			_, err = DecodeFloat64(buf, &m.F8)
			if err != nil {
				return err
			}
		case 9:
			_, err = DecodeArray(buf, &m.F9)
			if err != nil {
				return err
			}
		case 10:
			_, err = DecodeMap(buf, &m.F10)
			if err != nil {
				return err
			}
		case 11:
			_, err = DecodeBytes(buf, &m.F11)
			if err != nil {
				return err
			}
		case 12:
			testStruct := &TestStruct{}
			err := testStruct.Unmarshal(buf)
			if err != nil {
				return err
			}
			m.F12 = testStruct
		default:
			_, err = DeSerializeBuf(buf, nil)
			if err != nil {
				return err
			}
		}
	}
	if buf.GetRPos() != endPos {
		return ErrWrongSize
	}
	return nil
}

func TestMessage(t *testing.T) {
	s := TestStruct{}
	s.F1 = "testString"
	s.F2 = true
	s.F3 = 125
	s.F4 = 126
	s.F5 = 256
	s.F6 = 4096
	s.F7 = 12.1
	s.F8 = 1000000000000.12
	s.F9 = []string{"1", "2", "3"}
	f10 := make(map[string]string)
	f10["key1"] = "value1"
	f10["key2"] = "value2"
	s.F10 = f10
	s.F11 = []byte{1, 2, 3, 4}

	buffer := core.NewBytesBuffer(4096)
	s.Marshal(buffer)

	flag, _ := buffer.ReadByte()
	assert.Equal(t, byte(TMessage), flag)
	ds := TestStruct{}
	ds.Unmarshal(buffer)
	bytes, _ := json.Marshal(&ds)
	fmt.Println(string(bytes))
}

// serialize && deserialize string
func TestMotanSerializeString(t *testing.T) {
	ser := &Serialization{}
	motanVerifyString("teststring", ser, t)
	motanVerifyString("t", ser, t)
	motanVerifyString("", ser, t)
}

func TestMotanSerializeStringMap(t *testing.T) {
	ser := &Serialization{}
	var m map[string]string
	motanVerifyMap(m, ser, t)
	m = make(map[string]string, 16)
	m["k1"] = "v1"
	m["k2"] = "v2"
	motanVerifyMap(m, ser, t)
}

func TestMotanSerializeMap(t *testing.T) {
	ser := &Serialization{}
	value := make([]interface{}, 0, 16)
	var m map[interface{}]interface{}
	m = make(map[interface{}]interface{}, 16)
	var ik, iv int64 = 123, 456 // must use int64 for value check

	m["k1"] = "v1"
	m["k2"] = "v2"
	m[ik] = iv
	m[true] = false

	a := make([]interface{}, 0, 16)
	a = append(a, "test")
	a = append(a, "asdf")
	m["sarray"] = a

	value = append(value, m)
	value = append(value, 3.1415)
	bytes, err := ser.SerializeMulti(value)
	if err != nil {
		t.Errorf("serialize multi map fail. err:%v\n", err)
	}
	nvalue, err := ser.DeSerializeMulti(bytes, nil)
	if err != nil {
		t.Errorf("deserialize multi map fail. nvalue:%v, err:%v\n", nvalue, err)
	}
	if len(value) != len(nvalue) {
		t.Errorf("deserialize multi map fail. nvalue:%v, err:%v\n", nvalue, err)
	}
	nmap := nvalue[0].(map[interface{}]interface{})
	for k, v := range nmap {
		if sa, ok := v.([]interface{}); ok {
			ra := m[k].([]interface{})
			for i, st := range sa {
				if ra[i] != st {
					t.Errorf("deserialize multi map fail. k: %+v, v:%+v, nv:%+v\n", k, m[k], v)
				}
			}
		} else {
			if m[k] != v {
				t.Errorf("deserialize multi map fail. k: %+v, v:%+v, nv:%+v\n", k, m[k], v)
			}
		}

	}
}

func TestMotanSerializeArray(t *testing.T) {
	ser := &Serialization{}
	// string array
	value := make([]interface{}, 0, 16)
	sa := make([]string, 0, 16)
	for i := 0; i < 20; i++ {
		sa = append(sa, "slkje"+strconv.Itoa(i))
	}

	value = append(value, sa)
	bytes, err := ser.SerializeMulti(value)

	if err != nil {
		t.Errorf("serialize array fail. err:%v\n", err)
	}
	nvalue, err := ser.DeSerializeMulti(bytes, nil)
	if err != nil {
		t.Errorf("deserialize array fail. nvalue:%v, err:%v\n", nvalue, err)
	}
	rsa := value[0].([]string)
	if len(rsa) != len(sa) {
		t.Errorf("deserialize array fail. nvalue:%v, err:%v\n", nvalue, err)
	}
	for i, ts := range sa {
		if rsa[i] != ts {
			t.Errorf("deserialize array fail. sa:%v, rsa:%v\n", sa, rsa)
		}
	}

	//interface{} array
	a := make([]interface{}, 0, 16)
	var m map[interface{}]interface{}
	m = make(map[interface{}]interface{}, 16)
	var ik, iv int64 = 123, 456 // must use int64 for value check

	m["k1"] = "v1"
	m["k2"] = "v2"
	m[ik] = iv
	m[true] = false

	a = append(a, "test")
	a = append(a, "asdf")
	a = append(a, m)
	a = append(a, 3.1415)
	value = make([]interface{}, 0, 16)
	value = append(value, a)

	bytes, err = ser.SerializeMulti(value)
	if err != nil {
		t.Errorf("serialize array fail. err:%v\n", err)
	}
	nvalue, err = ser.DeSerializeMulti(bytes, nil)
	if err != nil {
		t.Errorf("deserialize array fail. nvalue:%v, err:%v\n", nvalue, err)
	}
	ria := value[0].([]interface{})
	if len(ria) != len(a) {
		t.Errorf("deserialize array fail. nvalue:%v, err:%v\n", nvalue, err)
	}
	for i, ts := range a {
		if im, ok := ts.(map[interface{}]interface{}); ok {
			for mk, mv := range m {
				if im[mk] != mv {
					t.Errorf("deserialize array fail. m: %+v, im:%+v\n", m, im)
				}
			}
		} else {
			if ria[i] != ts {
				t.Errorf("deserialize array fail. sa:%v, rsa:%v\n", sa, rsa)
			}
		}

	}
}

func TestMotanSerializeBytes(t *testing.T) {
	ser := &Serialization{}
	var ba []byte
	motanVerifyBytes(ba, ser, t)
	ba = make([]byte, 2, 2)
	motanVerifyBytes(ba, ser, t)
	ba = make([]byte, 0, 2)
	motanVerifyBytes(ba, ser, t)
	ba = append(ba, 3)
	ba = append(ba, '3')
	ba = append(ba, 'x')
	ba = append(ba, 0x12)
	motanVerifyBytes(ba, ser, t)
}

func TestMotanSerializeNil(t *testing.T) {
	ser := &Serialization{}
	var test error
	b, err := ser.Serialize(test)
	if err != nil {
		t.Errorf("serialize nil fail. err:%v\n", err)
	}
	if len(b) != 1 && b[0] != 0 {
		t.Errorf("serialize nil fail. b:%v\n", b)
	}
	var ntest *error
	_, err = ser.DeSerialize(b, ntest)
	if err != nil || ntest != nil {
		t.Errorf("serialize nil fail. err:%v, test:%v\n", err, test)
	}
}

func TestMotanSerializeMulti(t *testing.T) {
	//single value
	ser := &Serialization{}
	var rs string
	motanVerifySingleValue("string", &rs, ser, t)
	m := make(map[string]string, 16)
	m["k"] = "v"
	var rm map[string]string
	motanVerifySingleValue(m, &rm, ser, t)
	motanVerifySingleValue(nil, nil, ser, t)
	b := []byte{1, 2, 3}
	var rb []byte
	motanVerifySingleValue(b, &rb, ser, t)

	//multi value
	a := []interface{}{"stringxx", m, b, nil}
	r := []interface{}{&rs, &rm, &rb, nil}
	motanVerifyMulti(a, r, ser, t)

}

func TestMotanSerializeBaseType(t *testing.T) {
	ser := &Serialization{}
	// bool
	motanVerifyBaseType(true, ser, t)
	motanVerifyBaseType(false, ser, t)
	//byte
	motanVerifyBaseType(byte(math.MaxUint8), ser, t)
	motanVerifyBaseType(byte(16), ser, t)
	motanVerifyBaseType(byte(0), ser, t)
	motanVerifyBaseType(byte(255), ser, t)
	// int16
	motanVerifyBaseType(int16(math.MaxInt16), ser, t)
	motanVerifyBaseType(int16(math.MinInt16), ser, t)
	motanVerifyBaseType(int16(-16), ser, t)
	motanVerifyBaseType(int16(0), ser, t)

	//int32
	motanVerifyBaseType(int32(math.MaxInt32), ser, t)
	motanVerifyBaseType(int32(math.MinInt32), ser, t)
	motanVerifyBaseType(int32(-16), ser, t)
	motanVerifyBaseType(int32(0), ser, t)
	//int
	motanVerifyBaseType(int(-16), ser, t)
	motanVerifyBaseType(int(0), ser, t)
	//int64
	motanVerifyBaseType(int64(math.MaxInt64), ser, t)
	motanVerifyBaseType(int64(math.MinInt64), ser, t)
	motanVerifyBaseType(int64(-16), ser, t)
	motanVerifyBaseType(int64(0), ser, t)
	//float32
	motanVerifyBaseType(float32(math.MaxFloat32), ser, t)
	motanVerifyBaseType(float32(3.141592653), ser, t)
	motanVerifyBaseType(float32(-3.141592653), ser, t)
	motanVerifyBaseType(float32(0), ser, t)
	//float64
	motanVerifyBaseType(float64(math.MaxFloat64), ser, t)
	motanVerifyBaseType(float64(3.141592653), ser, t)
	motanVerifyBaseType(float64(-3.141592653), ser, t)
	motanVerifyBaseType(float64(0), ser, t)
}

func motanVerifyBaseType(v interface{}, s core.Serialization, t *testing.T) {
	sv, err := s.Serialize(v)
	if err != nil || len(sv) == 0 {
		t.Errorf("serialize fail. byte size:%d, err:%v\n", len(sv), err)
	}
	dv, err := s.DeSerialize(sv, reflect.TypeOf(v))
	// int should cast to int64; uint should cast to uint64
	if iv, ok := v.(int); ok {
		v = int64(iv)
	} else if uv, ok := v.(uint); ok {
		v = uint64(uv)
	}
	if err != nil {
		t.Errorf("serialize fail. err:%v\n", err)
	} else if v != dv {
		t.Errorf("deserialize value not correct. result:%v(%v), %v(%v)\n", reflect.TypeOf(v), v, reflect.TypeOf(dv), dv)
	}
}

func motanVerifySingleValue(i interface{}, reply interface{}, ser core.Serialization, t *testing.T) {
	a := []interface{}{i}
	b, err := ser.SerializeMulti(a)
	if err != nil {
		t.Errorf("serialize multi fail. err:%v\n", err)
	}
	if len(b) < 1 {
		t.Errorf("serialize multi fail. b:%v\n", b)
	}
	na := []interface{}{reply}
	v, err := ser.DeSerializeMulti(b, na)
	fmt.Printf("format:%+v\n", v[0])
	if err != nil {
		t.Errorf("serialize multi fail. err:%v\n", err)
	}

	if len(na) != 1 {
		t.Errorf("serialize multi fail. a:%v, na:%v\n", a, na)
	}
}

func motanVerifyMulti(v []interface{}, reply []interface{}, ser core.Serialization, t *testing.T) {
	b, err := ser.SerializeMulti(v)
	if err != nil {
		t.Errorf("serialize multi fail. err:%v\n", err)
	}
	if len(b) < 1 {
		t.Errorf("serialize multi fail. b:%v\n", b)
	}

	result, err := ser.DeSerializeMulti(b, reply)
	fmt.Printf("format:%+v\n", result)
	if err != nil {
		t.Errorf("serialize multi fail. err:%v\n", err)
	}
	if len(reply) != len(v) {
		t.Errorf("serialize multi fail. len:%d\n", len(reply))
	}

}

func motanVerifyString(s string, ser core.Serialization, t *testing.T) {
	b, err := ser.Serialize(s)
	if err != nil {
		t.Errorf("serialize string fail. err:%v\n", err)
	}
	if b[0] != TString {
		t.Errorf("serialize string fail. b:%v\n", b)
	}
	var ns string
	_, err = ser.DeSerialize(b, &ns)
	if err != nil {
		t.Errorf("serialize string fail. err:%v\n", err)
	}
	if ns != s {
		t.Errorf("serialize string fail. s:%s, ns:%s\n", s, ns)
	}
}

func motanVerifyMap(m map[string]string, ser core.Serialization, t *testing.T) {
	b, err := ser.Serialize(m)
	if err != nil {
		t.Errorf("serialize Map fail. err:%v\n", err)
	}
	if b[0] != TMap {
		t.Errorf("serialize Map fail. b:%v\n", b)
	}
	nm := make(map[string]string, 16)
	_, err = ser.DeSerialize(b, &nm)
	if err != nil {
		t.Errorf("serialize map fail. err:%v\n", err)
	}
	if len(nm) != len(m) {
		t.Errorf("serialize map fail. m:%s, nm:%s\n", m, nm)
	}
	for k, v := range nm {
		if v != m[k] {
			t.Errorf("serialize map value fail. m:%s, nm:%s\n", m, nm)
		}
	}
}

func motanVerifyBytes(ba []byte, ser core.Serialization, t *testing.T) {
	b, err := ser.Serialize(ba)
	if err != nil {
		t.Errorf("serialize []byte fail. err:%v\n", err)
	}
	if b[0] != TByteArray {
		t.Errorf("serialize []byte fail. b:%v\n", b)
	}
	nba := make([]byte, 0, 1024)
	_, err = ser.DeSerialize(b, &nba)
	if err != nil {
		t.Errorf("serialize []byte fail. err:%v\n", err)
	}
	if len(nba) != len(ba) {
		t.Errorf("serialize []byte fail. ba:%v, nba:%v\n", ba, nba)
	}
	for i, u := range nba {
		if u != ba[i] {
			t.Errorf("serialize []byte value fail. ba:%v, nba:%v\n", ba, nba)
		}
	}
}
