package pipeline

import (
	"bytes"
	"reflect"
	"regexp"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"

	"github.com/smartcontractkit/chainlink/core/utils"
)

var (
	ethABIRegex     = regexp.MustCompile(`\A\s*([a-zA-Z0-9_]+)?\s*\(\s*([a-zA-Z0-9\[\]_\s,]+\s*)?\)`)
	indexedKeyword  = []byte("indexed")
	calldataKeyword = []byte("calldata")
	memoryKeyword   = []byte("memory")
	storageKeyword  = []byte("storage")
	spaceDelim      = []byte(" ")
	commaDelim      = []byte(",")
)

func parseETHABIArgsString(theABI []byte, isLog bool) (args abi.Arguments, indexedArgs abi.Arguments, _ error) {
	var argStrs [][]byte
	if len(bytes.TrimSpace(theABI)) > 0 {
		argStrs = bytes.Split(theABI, commaDelim)
	}

	for _, argStr := range argStrs {
		argStr = bytes.Replace(argStr, calldataKeyword, nil, -1) // Strip `calldata` modifiers
		argStr = bytes.Replace(argStr, memoryKeyword, nil, -1)   // Strip `memory` modifiers
		argStr = bytes.Replace(argStr, storageKeyword, nil, -1)  // Strip `storage` modifiers
		argStr = bytes.TrimSpace(argStr)
		parts := bytes.Split(argStr, spaceDelim)

		var (
			argParts [][]byte
			typeStr  []byte
			argName  []byte
			indexed  bool
		)
		for i := range parts {
			parts[i] = bytes.TrimSpace(parts[i])
			if len(parts[i]) > 0 {
				argParts = append(argParts, parts[i])
			}
		}
		switch len(argParts) {
		case 0:
			return nil, nil, errors.Errorf("bad ABI specification, empty argument: %v", theABI)

		case 1:
			return nil, nil, errors.Errorf("bad ABI specification, missing argument name: %v", theABI)

		case 2:
			if isLog && bytes.Equal(argParts[1], indexedKeyword) {
				return nil, nil, errors.Errorf("bad ABI specification, missing argument name: %v", theABI)
			}
			typeStr = argParts[0]
			argName = argParts[1]

		case 3:
			if !isLog {
				return nil, nil, errors.Errorf("bad ABI specification, too many components in argument: %v", theABI)
			} else if bytes.Equal(argParts[0], indexedKeyword) || bytes.Equal(argParts[2], indexedKeyword) {
				return nil, nil, errors.Errorf("bad ABI specification, 'indexed' keyword must appear between argument type and name: %v", theABI)
			} else if !bytes.Equal(argParts[1], indexedKeyword) {
				return nil, nil, errors.Errorf("bad ABI specification, unknown keyword '%v' between argument type and name: %v", string(argParts[1]), theABI)
			}
			typeStr = argParts[0]
			argName = argParts[2]
			indexed = true

		default:
			return nil, nil, errors.Errorf("bad ABI specification, too many components in argument: %v", theABI)
		}
		typ, err := abi.NewType(string(typeStr), "", nil)
		if err != nil {
			return nil, nil, errors.Errorf("bad ABI specification: %v", err.Error())
		}
		args = append(args, abi.Argument{Type: typ, Name: string(argName), Indexed: indexed})
		if indexed {
			indexedArgs = append(indexedArgs, abi.Argument{Type: typ, Name: string(argName), Indexed: indexed})
		}
	}
	return args, indexedArgs, nil
}

func parseETHABIString(theABI []byte, isLog bool) (name string, args abi.Arguments, indexedArgs abi.Arguments, err error) {
	matches := ethABIRegex.FindAllSubmatch(theABI, -1)
	if len(matches) != 1 || len(matches[0]) != 3 {
		return "", nil, nil, errors.Errorf("bad ABI specification: %v", theABI)
	}
	name = string(bytes.TrimSpace(matches[0][1]))
	args, indexedArgs, err = parseETHABIArgsString(matches[0][2], isLog)
	return name, args, indexedArgs, err
}

func convertToETHABIType(val interface{}, abiType abi.Type) (interface{}, error) {
	srcVal := reflect.ValueOf(val)

	if abiType.GetType() == srcVal.Type() {
		return val, nil
	}

	switch abiType.T {
	case abi.IntTy, abi.UintTy:
		return convertToETHABIInteger(val, abiType)

	case abi.StringTy:
		switch val := val.(type) {
		case string:
			return val, nil
		case []byte:
			return string(val), nil
		}

	case abi.BytesTy:
		switch val := val.(type) {
		case string:
			if len(val) > 1 && val[:2] == "0x" {
				return hexutil.Decode(val)
			}
			return []byte(val), nil
		case []byte:
			return val, nil
		default:
			return convertToETHABIBytes(abiType.GetType(), srcVal, srcVal.Len())
		}

	case abi.FixedBytesTy:
		destType := abiType.GetType()
		return convertToETHABIBytes(destType, srcVal, destType.Len())

	case abi.AddressTy:
		switch val := val.(type) {
		case common.Address:
			return val, nil
		case [20]byte:
			return common.Address(val), nil
		default:
			maybeBytes, err := convertToETHABIBytes(bytes20Type, srcVal, 20)
			if err != nil {
				return nil, err
			}
			bs, ok := maybeBytes.([20]byte)
			if !ok {
				panic("impossible")
			}
			return common.Address(bs), nil
		}

	case abi.BoolTy:
		switch val := val.(type) {
		case bool:
			return val, nil
		case string:
			return strconv.ParseBool(val)
		}

	case abi.SliceTy:
		dest := reflect.MakeSlice(abiType.GetType(), srcVal.Len(), srcVal.Len())
		for i := 0; i < dest.Len(); i++ {
			elem, err := convertToETHABIType(srcVal.Index(i).Interface(), *abiType.Elem)
			if err != nil {
				return nil, err
			}
			dest.Index(i).Set(reflect.ValueOf(elem))
		}
		return dest.Interface(), nil

	case abi.ArrayTy:
		if srcVal.Kind() != reflect.Slice && srcVal.Kind() != reflect.Array {
			return nil, errors.Wrapf(ErrBadInput, "cannot convert %v to %v", srcVal.Type(), abiType)
		} else if srcVal.Len() != abiType.Size {
			return nil, errors.Wrapf(ErrBadInput, "incorrect length: expected %v, got %v", abiType.Size, srcVal.Len())
		}

		dest := reflect.New(abiType.GetType()).Elem()
		for i := 0; i < dest.Len(); i++ {
			elem, err := convertToETHABIType(srcVal.Index(i).Interface(), *abiType.Elem)
			if err != nil {
				return nil, err
			}
			dest.Index(i).Set(reflect.ValueOf(elem))
		}
		return dest.Interface(), nil

	case abi.TupleTy:
		panic("not supported")
	}
	return nil, errors.Wrapf(ErrBadInput, "cannot convert %v to %v", srcVal.Type(), abiType)
}

func convertToETHABIBytes(destType reflect.Type, srcVal reflect.Value, length int) (interface{}, error) {
	switch srcVal.Type().Kind() {
	case reflect.Slice:
		if destType.Len() != length {
			return nil, errors.Wrapf(ErrBadInput, "incorrect length: expected %v, got %v", length, destType.Len())
		} else if srcVal.Type().Elem().Kind() != reflect.Uint8 {
			return nil, errors.Wrapf(ErrBadInput, "cannot convert %v to %v", srcVal.Type(), destType)
		}
		destVal := reflect.MakeSlice(destType, length, length)
		reflect.Copy(destVal.Slice(0, length), srcVal.Slice(0, srcVal.Len()))
		return destVal.Interface(), nil

	case reflect.Array:
		if destType.Len() != length {
			return nil, errors.Wrapf(ErrBadInput, "incorrect length: expected %v, got %v", length, destType.Len())
		} else if srcVal.Type().Elem().Kind() != reflect.Uint8 {
			return nil, errors.Wrapf(ErrBadInput, "cannot convert %v to %v", srcVal.Type(), destType)
		}
		destVal := reflect.New(destType).Elem()
		reflect.Copy(destVal.Slice(0, length), srcVal.Slice(0, srcVal.Len()))
		return destVal.Interface(), nil

	case reflect.String:
		s := srcVal.Convert(stringType).Interface().(string)
		if s[:2] == "0x" {
			if len(s) != (length*2)+2 {
				return nil, errors.Wrapf(ErrBadInput, "incorrect length: expected %v, got %v", length, destType.Len())
			}
			return hexutil.Decode(s)
		}

		if destType.Len() != length {
			return nil, errors.Wrapf(ErrBadInput, "incorrect length: expected %v, got %v", length, destType.Len())
		}
		return convertToETHABIBytes(destType, srcVal.Convert(bytesType), length)

	default:
		return nil, errors.Wrapf(ErrBadInput, "cannot convert %v to %v", srcVal.Type(), destType)
	}
}

var ErrOverflow = errors.New("overflow")

func convertToETHABIInteger(val interface{}, abiType abi.Type) (interface{}, error) {
	d, err := utils.ToDecimal(val)
	if err != nil {
		return nil, err
	}

	i := d.BigInt()

	if abiType.Size > 64 {
		return i, nil
	}

	converted := reflect.New(abiType.GetType()).Elem()
	if i.IsUint64() {
		if converted.OverflowUint(i.Uint64()) {
			return nil, ErrOverflow
		}
		converted.SetUint(i.Uint64())

	} else {
		if converted.OverflowInt(i.Int64()) {
			return nil, ErrOverflow
		}
		converted.SetInt(i.Int64())
	}
	return converted.Interface(), nil
}
