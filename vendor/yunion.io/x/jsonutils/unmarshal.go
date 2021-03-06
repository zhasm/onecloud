// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jsonutils

/**
jsonutils.Unmarshal

Fill the value of JSONObject into any object

*/

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"yunion.io/x/log"
	"yunion.io/x/pkg/gotypes"
	"yunion.io/x/pkg/tristate"
	"yunion.io/x/pkg/util/reflectutils"
	"yunion.io/x/pkg/util/timeutils"
	"yunion.io/x/pkg/utils"
)

func (this *JSONValue) Unmarshal(obj interface{}, keys ...string) error {
	return ErrUnsupported
}

func (this *JSONArray) Unmarshal(obj interface{}, keys ...string) error {
	return jsonUnmarshal(this, obj, keys)
}

func (this *JSONDict) Unmarshal(obj interface{}, keys ...string) error {
	return jsonUnmarshal(this, obj, keys)
}

func jsonUnmarshal(jo JSONObject, o interface{}, keys []string) error {
	if len(keys) > 0 {
		var err error = nil
		jo, err = jo.Get(keys...)
		if err != nil {
			return errors.Wrap(err, "Get")
		}
	}
	value := reflect.Indirect(reflect.ValueOf(o))
	return jo.unmarshalValue(value)
}

func (this *JSONValue) unmarshalValue(val reflect.Value) error {
	if val.CanSet() {
		zeroVal := reflect.New(val.Type()).Elem()
		val.Set(zeroVal)
	}
	return nil
}

func (this *JSONInt) unmarshalValue(val reflect.Value) error {
	switch val.Type() {
	case JSONIntType:
		json := val.Interface().(JSONInt)
		json.data = this.data
		return nil
	case JSONIntPtrType, JSONObjectType:
		val.Set(reflect.ValueOf(this))
		return nil
	case JSONStringType:
		json := val.Interface().(JSONString)
		json.data = fmt.Sprintf("%d", this.data)
		return nil
	case JSONStringPtrType:
		json := val.Interface().(*JSONString)
		data := fmt.Sprintf("%d", this.data)
		if json == nil {
			json = NewString(data)
			val.Set(reflect.ValueOf(json))
		} else {
			json.data = data
		}
		return nil
	case JSONBoolType, JSONFloatType, JSONArrayType, JSONDictType, JSONBoolPtrType, JSONFloatPtrType, JSONArrayPtrType, JSONDictPtrType:
		return ErrTypeMismatch // fmt.Errorf("JSONInt type mismatch %s", val.Type())
	case tristate.TriStateType:
		if this.data == 0 {
			val.Set(tristate.TriStateFalseValue)
		} else {
			val.Set(tristate.TriStateTrueValue)
		}
	}
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64:
		val.SetInt(this.data)
	case reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		val.SetUint(uint64(this.data))
	case reflect.Float32, reflect.Float64:
		val.SetFloat(float64(this.data))
	case reflect.Bool:
		if this.data == 0 {
			val.SetBool(false)
		} else {
			val.SetBool(true)
		}
	case reflect.String:
		val.SetString(fmt.Sprintf("%d", this.data))
	case reflect.Ptr:
		if val.IsNil() {
			val.Set(reflect.New(val.Type().Elem()))
		}
		return this.unmarshalValue(val.Elem())
	case reflect.Interface:
		val.Set(reflect.ValueOf(this.data))
	default:
		log.Errorf("JSONDict type mismatch: %s", val.Type())
		return ErrTypeMismatch // fmt.Errorf("JSONInt type mismatch: %s", val.Type())
	}
	return nil
}

func (this *JSONBool) unmarshalValue(val reflect.Value) error {
	switch val.Type() {
	case JSONBoolType:
		json := val.Interface().(JSONBool)
		json.data = this.data
		return nil
	case JSONBoolPtrType, JSONObjectType:
		val.Set(reflect.ValueOf(this))
		return nil
	case JSONStringType:
		json := val.Interface().(JSONString)
		json.data = strconv.FormatBool(this.data)
		return nil
	case JSONStringPtrType:
		json := val.Interface().(*JSONString)
		data := strconv.FormatBool(this.data)
		if json == nil {
			json = NewString(data)
			val.Set(reflect.ValueOf(json))
		} else {
			json.data = data
		}
		return nil
	case JSONIntType, JSONFloatType, JSONArrayType, JSONDictType, JSONIntPtrType, JSONFloatPtrType, JSONArrayPtrType, JSONDictPtrType:
		return ErrTypeMismatch // fmt.Errorf("JSONBool type mismatch %s", val.Type())
	case tristate.TriStateType:
		if this.data {
			val.Set(tristate.TriStateTrueValue)
		} else {
			val.Set(tristate.TriStateFalseValue)
		}
	}
	switch val.Kind() {
	case reflect.Int, reflect.Uint, reflect.Int8, reflect.Uint8,
		reflect.Int16, reflect.Uint16, reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64:
		if this.data {
			val.SetInt(1)
		} else {
			val.SetInt(0)
		}
	case reflect.Float32, reflect.Float64:
		if this.data {
			val.SetFloat(1.0)
		} else {
			val.SetFloat(0.0)
		}
	case reflect.Bool:
		val.SetBool(this.data)
	case reflect.String:
		if this.data {
			val.SetString("true")
		} else {
			val.SetString("false")
		}
	case reflect.Ptr:
		if val.IsNil() {
			val.Set(reflect.New(val.Type().Elem()))
		}
		return this.unmarshalValue(val.Elem())
	case reflect.Interface:
		val.Set(reflect.ValueOf(this.data))
	default:
		log.Errorf("JSONDict type mismatch: %s", val.Type())
		return ErrTypeMismatch // fmt.Errorf("JSONBool type mismatch: %s", val.Type())
	}
	return nil
}

func (this *JSONFloat) unmarshalValue(val reflect.Value) error {
	switch val.Type() {
	case JSONFloatType:
		json := val.Interface().(JSONFloat)
		json.data = this.data
		return nil
	case JSONFloatPtrType, JSONObjectType:
		val.Set(reflect.ValueOf(this))
		return nil
	case JSONStringType:
		json := val.Interface().(JSONString)
		json.data = fmt.Sprintf("%f", this.data)
		return nil
	case JSONStringPtrType:
		json := val.Interface().(*JSONString)
		data := fmt.Sprintf("%f", this.data)
		if json == nil {
			json = NewString(data)
			val.Set(reflect.ValueOf(json))
		} else {
			json.data = data
		}
		return nil
	case JSONIntType:
		json := val.Interface().(JSONInt)
		json.data = int64(this.data)
		return nil
	case JSONIntPtrType:
		json := val.Interface().(*JSONInt)
		if json == nil {
			json = NewInt(int64(this.data))
			val.Set(reflect.ValueOf(json))
		} else {
			json.data = int64(this.data)
		}
		return nil
	case JSONBoolType:
		json := val.Interface().(JSONBool)
		json.data = (int(this.data) != 0)
		return nil
	case JSONArrayType, JSONDictType, JSONBoolPtrType, JSONArrayPtrType, JSONDictPtrType:
		return ErrTypeMismatch // fmt.Errorf("JSONFloat type mismatch %s", val.Type())
	case tristate.TriStateType:
		if int(this.data) == 0 {
			val.Set(tristate.TriStateFalseValue)
		} else {
			val.Set(tristate.TriStateTrueValue)
		}
	}
	switch val.Kind() {
	case reflect.Int, reflect.Uint, reflect.Int8, reflect.Uint8,
		reflect.Int16, reflect.Uint16, reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64:
		val.SetInt(int64(this.data))
	case reflect.Float32, reflect.Float64:
		val.SetFloat(this.data)
	case reflect.Bool:
		if this.data == 0 {
			val.SetBool(false)
		} else {
			val.SetBool(true)
		}
	case reflect.String:
		val.SetString(fmt.Sprintf("%f", this.data))
	case reflect.Ptr:
		if val.IsNil() {
			val.Set(reflect.New(val.Type().Elem()))
		}
		return this.unmarshalValue(val.Elem())
	case reflect.Interface:
		val.Set(reflect.ValueOf(this.data))
	default:
		log.Errorf("JSONDict type mismatch: %s", val.Type())
		return ErrTypeMismatch // fmt.Errorf("JSONFloat type mismatch: %s", val.Type())
	}
	return nil
}

func (this *JSONString) unmarshalValue(val reflect.Value) error {
	switch val.Type() {
	case JSONStringType:
		json := val.Interface().(JSONString)
		json.data = this.data
		return nil
	case JSONStringPtrType, JSONObjectType:
		val.Set(reflect.ValueOf(this))
		return nil
	case gotypes.TimeType:
		var tm time.Time
		var err error
		if len(this.data) > 0 {
			tm, err = timeutils.ParseTimeStr(this.data)
			if err != nil {
				return errors.Wrap(err, "timeutils.ParseTimeStr")
			}
		} else {
			tm = time.Time{}
		}
		val.Set(reflect.ValueOf(tm))
		return nil
	case JSONBoolType:
		json := val.Interface().(JSONBool)
		switch strings.ToLower(this.data) {
		case "true", "yes", "on", "1":
			json.data = true
		default:
			json.data = false
		}
		return nil
	case JSONBoolPtrType:
		json := val.Interface().(*JSONBool)
		var data bool
		switch strings.ToLower(this.data) {
		case "true", "yes", "on", "1":
			data = true
		default:
			data = false
		}
		if json == nil {
			json = &JSONBool{data: data}
		} else {
			json.data = data
		}
		return nil
	case JSONIntType, JSONFloatType, JSONArrayType, JSONDictType,
		JSONBoolPtrType, JSONIntPtrType, JSONFloatPtrType, JSONArrayPtrType, JSONDictPtrType:
		return ErrTypeMismatch // fmt.Errorf("JSONString type mismatch %s", val.Type())
	case tristate.TriStateType:
		switch strings.ToLower(this.data) {
		case "true", "yes", "on", "1":
			val.Set(tristate.TriStateTrueValue)
		case "false", "no", "off", "0":
			val.Set(tristate.TriStateFalseValue)
		default:
			val.Set(tristate.TriStateNoneValue)
		}
	}
	switch val.Kind() {
	case reflect.Int, reflect.Uint, reflect.Int8, reflect.Uint8,
		reflect.Int16, reflect.Uint16, reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64:
		intVal, err := strconv.ParseInt(this.data, 10, 64)
		if err != nil {
			return err
		}
		val.SetInt(intVal)
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(normalizeCurrencyString(this.data), 64)
		if err != nil {
			return err
		}
		val.SetFloat(floatVal)
	case reflect.Bool:
		val.SetBool(utils.ToBool(this.data))
	case reflect.String:
		val.SetString(this.data)
	case reflect.Ptr:
		if val.IsNil() {
			val.Set(reflect.New(val.Type().Elem()))
		}
		return this.unmarshalValue(val.Elem())
	case reflect.Interface:
		val.Set(reflect.ValueOf(this.data))
	default:
		log.Errorf("JSONDict type mismatch: %s", val.Type())
		return ErrTypeMismatch // fmt.Errorf("JSONString type mismatch: %s", val.Type())
	}
	return nil
}

func (this *JSONArray) unmarshalValue(val reflect.Value) error {
	switch val.Type() {
	case JSONArrayType:
		array := val.Interface().(JSONArray)
		if this.data != nil {
			array.Add(this.data...)
		}
		val.Set(reflect.ValueOf(array))
		return nil
	case JSONArrayPtrType, JSONObjectType:
		val.Set(reflect.ValueOf(this))
		return nil
	case JSONDictType, JSONIntType, JSONStringType, JSONBoolType, JSONFloatType,
		JSONDictPtrType, JSONIntPtrType, JSONStringPtrType, JSONBoolPtrType, JSONFloatPtrType:
		return ErrTypeMismatch //fmt.Errorf("JSONArray type mismatch %s", val.Type())
	}
	switch val.Kind() {
	case reflect.String:
		val.SetString(this.String())
		return nil
	case reflect.Ptr:
		kind := val.Type().Elem().Kind()
		if kind == reflect.Array || kind == reflect.Slice {
			if val.IsNil() {
				val.Set(reflect.New(val.Type().Elem()))
			}
			return this.unmarshalValue(val.Elem())
		}
		return ErrTypeMismatch // fmt.Errorf("JSONArray type mismatch %s", val.Type())
	case reflect.Interface:
		val.Set(reflect.ValueOf(this.data))
	case reflect.Slice, reflect.Array:
		if val.Kind() == reflect.Array {
			if val.Len() != len(this.data) {
				return ErrArrayLengthMismatch // fmt.Errorf("JSONArray length unmatch %s: %d != %d", val.Type(), val.Len(), len(this.data))
			}
		} else if val.Kind() == reflect.Slice {
			dataLen := len(this.data)
			if val.Cap() < dataLen {
				newVal := reflect.MakeSlice(val.Type(), dataLen, dataLen)
				val.Set(newVal)
			} else if val.Len() != dataLen {
				val.SetLen(dataLen)
			}
		}
		for i, json := range this.data {
			err := json.unmarshalValue(val.Index(i))
			if err != nil {
				return errors.Wrap(err, "unmarshalValue")
			}
		}
	default:
		log.Errorf("JSONDict type mismatch: %s", val.Type())
		return ErrTypeMismatch // fmt.Errorf("JSONArray type mismatch: %s", val.Type())
	}
	return nil
}

func (this *JSONDict) unmarshalValue(val reflect.Value) error {
	switch val.Type() {
	case JSONDictType:
		dict := val.Interface().(JSONDict)
		dict.Update(this)
		return nil
	case JSONDictPtrType, JSONObjectType:
		val.Set(reflect.ValueOf(this))
		return nil
	case JSONArrayType, JSONIntType, JSONBoolType, JSONFloatType, JSONStringType,
		JSONArrayPtrType, JSONIntPtrType, JSONBoolPtrType, JSONFloatPtrType, JSONStringPtrType:
		return ErrTypeMismatch // fmt.Errorf("JSONDict type mismatch %s", val.Type())
	}
	switch val.Kind() {
	case reflect.String:
		val.SetString(this.String())
		return nil
	case reflect.Map:
		return this.unmarshalMap(val)
	case reflect.Struct:
		return this.unmarshalStruct(val)
	case reflect.Interface:
		if val.Type().Implements(gotypes.ISerializableType) {
			objPtr, err := gotypes.NewSerializable(val.Type())
			if err != nil {
				return err
			}
			if objPtr == nil {
				val.Set(reflect.ValueOf(this.data))
				return nil
			}
			err = this.unmarshalValue(reflect.ValueOf(objPtr))
			if err != nil {
				return errors.Wrap(err, "unmarshalValue")
			}
			//
			// XXX
			//
			// cannot unmarshal nested anonymous interface
			// as nested anonymous interface is treated as a named field
			// please use jsonutils.Deserialize to descrialize such interface
			// ...
			// objPtr = gotypes.Transform(val.Type(), objPtr)
			//
			val.Set(reflect.ValueOf(objPtr).Convert(val.Type()))
		} else {
			log.Errorf("Do not known how to deserialize json into this interface type %s", val.Type())
			return ErrInterfaceUnsupported
		}
	case reflect.Ptr:
		kind := val.Type().Elem().Kind()
		if kind == reflect.Struct || kind == reflect.Map {
			if val.IsNil() {
				newVal := reflect.New(val.Type().Elem())
				val.Set(newVal)
			}
			return this.unmarshalValue(val.Elem())
		}
		fallthrough
	default:
		log.Errorf("JSONDict type mismatch: %s", val.Type())
		return ErrTypeMismatch
	}
	return nil
}

func (this *JSONDict) unmarshalMap(val reflect.Value) error {
	if val.IsNil() {
		mapVal := reflect.MakeMap(val.Type())
		val.Set(mapVal)
	}
	valType := val.Type()
	keyType := valType.Key()
	if keyType.Kind() != reflect.String {
		return ErrMapKeyMustString // fmt.Errorf("map key must be string")
	}
	for k, v := range this.data {
		keyVal := reflect.ValueOf(k)
		valVal := reflect.New(valType.Elem()).Elem()

		err := v.unmarshalValue(valVal)
		if err != nil {
			log.Debugf("unmarshalMap field %s error %s", k, err)
			return errors.Wrap(err, "unmarshalValue")
		}
		val.SetMapIndex(keyVal, valVal)
	}
	return nil
}

func (this *JSONDict) unmarshalStruct(val reflect.Value) error {
	fieldValues := reflectutils.FetchStructFieldValueSetForWrite(val)
	for k, v := range this.data {
		fieldValue, find := fieldValues.GetValue(k)
		if find {
			err := v.unmarshalValue(fieldValue)
			if err != nil {
				log.Debugf("unmarshalStruct field %s error %s", k, err)
				return errors.Wrap(err, "unmarshalValue")
			}
		}
	}
	return nil
}
