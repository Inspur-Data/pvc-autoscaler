package utils

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/henrylee2cn/mahonia"
)

func IsInt(s interface{}) bool {
	_, e := strconv.ParseInt(ToString(s), 10, 64)

	return e == nil
}

func IsFloat(s interface{}) bool {
	_, e := strconv.ParseFloat(ToString(s), 64)

	return e == nil
}

func IsArray(v interface{}) bool {
	if IsEmpty(v) {
		return false
	}
	switch reflect.TypeOf(v).Kind() {
	case reflect.Array, reflect.Slice:
		return true
	default:
		return false
	}
}

func IsMapArray(v interface{}) bool {
	a, b := v.([]interface{})
	if b {
		for _, m := range a {
			switch m.(type) {
			case map[string]interface{}:
				return true
			default:
				return false
			}
		}
	}

	return false
}

func IsJson(b []byte) bool {
	var j json.RawMessage

	return json.Unmarshal(b, &j) == nil
}

func IsNil(v interface{}) bool {
	if v == nil {
		return true
	}

	vi := reflect.ValueOf(v)
	if vi.Kind() == reflect.Ptr {
		return vi.IsNil()
	}

	return false
}

func IsEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	switch v.(type) {
	case P:
		return len(v.(P)) == 0
	case []interface{}:
		return len(v.([]interface{})) == 0
	case []P:
		return len(v.([]P)) == 0
	case *[]P:
		return len(*v.(*[]P)) == 0
	case map[string][]interface{}:
		return len(v.(map[string][]interface{})) == 0
	case map[string]interface{}:
		return len(v.(map[string]interface{})) == 0
	}
	return ToString(v) == ""
}

func ToInt(s interface{}, defaultValue ...int) int {
	i, e := strconv.Atoi(ToString(s))
	if e != nil && len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return i
}

// 若存在小数部分，直接舍去小数
func ToIntRoundDown(s interface{}, defaultValue ...int) int {
	i, e := strconv.Atoi(toIntRoundDown(s))
	if e != nil && len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return i
}

func ToInt64(s interface{}, defaultValue ...int64) int64 {
	switch s.(type) {
	case int64:
		return s.(int64)
	case int:
		return int64(s.(int))
	case float64:
		return int64(s.(float64))
	}

	i64, e := strconv.ParseInt(ToString(s), 10, 64)
	if e != nil && len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return i64
}

// 若存在小数部分，直接舍去小数
func ToInt64RoundDown(s interface{}, defaultValue ...int64) int64 {
	switch s.(type) {
	case int64:
		return s.(int64)
	case int:
		return int64(s.(int))
	case float64:
		return int64(s.(float64))
	}

	i64, e := strconv.ParseInt(toIntRoundDown(s), 10, 64)
	if e != nil && len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return i64
}

func toIntRoundDown(s interface{}) string {
	str := ToString(s)
	if strings.Contains(str, ".") {
		_, e := strconv.ParseFloat(str, 64)
		if e != nil {
			return "0"
		}
		temp := strings.Split(str, ".")
		return temp[0]
	}
	return str
}

func ToFloat(s interface{}, defaultValue ...float64) float64 {
	f64, e := strconv.ParseFloat(ToString(s), 64)
	if e != nil && len(defaultValue) > 0 {
		return defaultValue[0]
	}

	return f64
}

func ToBool(s interface{}) (bool, error) {
	switch ToString(s) {
	case "1", "t", "T", "true", "TRUE", "True":
		return true, nil
	case "0", "f", "F", "false", "FALSE", "False":
		return false, nil
	}

	return false, fmt.Errorf("Parse Fial:%v", ToString(s))
}

func ToString(v interface{}, def ...string) string {
	if v != nil {
		switch v.(type) {
		case []byte:
			return string(v.([]byte))
		case *P, P:
			var p P
			switch v.(type) {
			case *P:
				if v.(*P) != nil {
					p = *v.(*P)
				}
			case P:
				p = v.(P)
			}
			var keys []string
			for k := range p {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			r := "P{"
			for _, k := range keys {
				r = JoinStr(r, k, ":", p[k], " ")
			}
			r = JoinStr(r, "}")
			return r
		case map[string]interface{}, []P, []interface{}:
			return JSONEncode(v)
		case int64:
			return strconv.FormatInt(v.(int64), 10)
		case []string:
			s := ""
			for _, j := range v.([]string) {
				s = JoinStr(s, ",", j)
			}
			if len(s) > 0 {
				s = s[1:]
			}
			return s
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	if len(def) > 0 {
		return def[0]
	} else {
		return ""
	}
}

func ToP(v interface{}) P {
	if v != nil {
		switch v.(type) {
		case P:
			return v.(P)
		case *P:
			return *v.(*P)
		case string:
			return JSONDecode(v.(string))
		case []byte:
			return JSONDecode(string(v.([]byte)))
		case map[string]interface{}:
			return v.(map[string]interface{})
		default:
			return P{}
		}
	}

	return P{}
}

func ToStrings(v interface{}) []string {
	strs := []string{}
	if v != nil {
		switch v.(type) {
		case []interface{}:
			for _, i := range v.([]interface{}) {
				strs = append(strs, ToString(i))
			}
		case []string:
			for _, i := range v.([]string) {
				strs = append(strs, i)
			}
		case string, interface{}:
			strs = append(strs, ToString(v))
		}
	}

	return strs
}

func GbkToUtf8(s []byte) []byte {
	utf8 := mahonia.NewDecoder("gbk").ConvertString(string(s))

	return []byte(utf8)
}

// XxYy to xx_yy
func CamelToSnake(camel string) string {
	snake := make([]byte, 0, len(camel)*2)
	j := false
	num := len(camel)
	for i := 0; i < num; i++ {
		d := camel[i]
		if i > 0 && d >= 'A' && d <= 'Z' && j {
			snake = append(snake, '_')
		}
		if d != '_' {
			j = true
		}
		snake = append(snake, d)
	}

	return strings.ToLower(string(snake[:]))
}

// any time to Nanos
func ToNanos(time int, sourceUnit string) int32 {
	switch sourceUnit {
	case "ms":
		return int32(time * 1e6)
	case "s":
		return int32(time * 1e9)
	case "min":
		return int32(time * 60 * 1e9)
	case "h":
		return int32(time * 60 * 60 * 1e9)
	default:
		return int32(time)
	}
}

// any time to Seconds
func ToSeconds(time int, sourceUnit string) int32 {
	switch sourceUnit {
	case "ms":
		return int32(time / 1e3)
	case "s":
		return int32(time)
	case "min":
		return int32(time * 60)
	case "h":
		return int32(time * 60 * 60)
	default:
		return int32(time)
	}
}

// split a time string to time number, unit, and determine if it's a time string.
func SplitTimeStr(timeStr string) (timeNum int, unit string, ifTimeStr bool) {
	if strings.HasSuffix(timeStr, "ms") {
		tmp := timeStr[0 : len(timeStr)-2]
		if IsInt(tmp) {
			return ToInt(tmp), "ms", true
		} else {
			return 0, "ErrorString", false
		}
	}
	if strings.HasSuffix(timeStr, "s") {
		tmp := timeStr[0 : len(timeStr)-1]
		if IsInt(tmp) {
			return ToInt(tmp), "s", true
		} else {
			return 0, "ErrorString", false
		}
	}
	if strings.HasSuffix(timeStr, "min") {
		tmp := timeStr[0 : len(timeStr)-3]
		if IsInt(tmp) {
			return ToInt(tmp), "min", true
		} else {
			return 0, "ErrorString", false
		}
	}
	if strings.HasSuffix(timeStr, "h") {
		tmp := timeStr[0 : len(timeStr)-1]
		if IsInt(tmp) {
			return ToInt(tmp), "h", true
		} else {
			return 0, "ErrorString", false
		}
	}
	return 0, "ErrorString", false
}

func IsEqual(s1, s2 interface{}) bool {
	str1 := ToString(s1)
	str2 := ToString(s2)
	if strings.Compare(str1, str2) == 0 {
		return true
	} else {
		return false
	}
}

func IntOrStringToInt(v intstr.IntOrString) int {
	if v.Type == 0 {
		return v.IntValue()
	} else {
		return ToInt(v.StrVal)
	}
}

func IntOrStringToStr(v intstr.IntOrString) string {
	if v.Type == 0 {
		return ToString(v.IntValue())
	} else {
		return v.StrVal
	}
}
func JoinStr(val ...interface{}) (r string) {
	for _, v := range val {
		r += ToString(v)
	}

	return
}
