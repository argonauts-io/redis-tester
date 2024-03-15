package resp_encoder

import (
	"fmt"
	"strconv"

	resp_value "github.com/codecrafters-io/redis-tester/internal/resp/value"
)

func Encode(v resp_value.Value) []byte {
	switch v.Type {
	case resp_value.INTEGER:
		return encodeInteger(v)
	case resp_value.SIMPLE_STRING:
		return encodeSimpleString(v)
	case resp_value.BULK_STRING:
		return encodeBulkString(v)
	case resp_value.RDB_BULK_STRING:
		return encodeRDBAsBulkString(v)
	case resp_value.ERROR:
		return encodeError(v)
	case resp_value.ARRAY:
		return encodeArray(v)
	default:
		panic(fmt.Sprintf("unsupported type: %v", v.Type))
	}
}

func encodeInteger(v resp_value.Value) []byte {
	int_value, err := strconv.Atoi(v.String())
	if err != nil {
		panic(err) // We only expect valid values to be passed in
	}

	return []byte(fmt.Sprintf(":%d\r\n", int_value))
}

func encodeSimpleString(v resp_value.Value) []byte {
	return []byte(fmt.Sprintf("+%s\r\n", v.String()))
}

func encodeBulkString(v resp_value.Value) []byte {
	return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(v.Bytes()), v.Bytes()))
}

func encodeRDBAsBulkString(v resp_value.Value) []byte {
	return []byte(fmt.Sprintf("$%d\r\n%s", len(v.Bytes()), v.Bytes()))
}

func encodeError(v resp_value.Value) []byte {
	return []byte(fmt.Sprintf("-%s\r\n", v.String()))
}

func encodeArray(v resp_value.Value) []byte {
	res := []byte{}

	for _, elem := range v.Array() {
		res = append(res, Encode(elem)...)
	}

	return []byte(fmt.Sprintf("*%d\r\n%s", len(v.Array()), res))
}
