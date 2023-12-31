package hash

import (
	"context"
	"errors"
	"fmt"
	"github.com/kelvinmwinuka/memstore/src/utils"
	"math/rand"
	"net"
	"slices"
	"strconv"
	"strings"
)

type Plugin struct {
	name        string
	commands    []utils.Command
	description string
}

func (p Plugin) Name() string {
	return p.name
}

func (p Plugin) Commands() []utils.Command {
	return p.commands
}

func (p Plugin) Description() string {
	return p.description
}

func handleHSET(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	if len(cmd) < 4 {
		return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
	}

	key := cmd[1]
	entries := make(map[string]interface{})

	if len(cmd[2:])%2 != 0 {
		return nil, errors.New("each field must have a corresponding value")
	}

	for i := 2; i <= len(cmd)-2; i += 2 {
		entries[cmd[i]] = utils.AdaptType(cmd[i+1])
	}

	if !server.KeyExists(key) {
		_, err := server.CreateKeyAndLock(ctx, key)
		if err != nil {
			return nil, err
		}
		defer server.KeyUnlock(key)
		server.SetValue(ctx, key, entries)
		return []byte(fmt.Sprintf(":%d\r\n\r\n", len(entries))), nil
	}

	_, err := server.KeyLock(ctx, key)
	if err != nil {
		return nil, err
	}
	defer server.KeyUnlock(key)

	hash, ok := server.GetValue(key).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value at %s is not a hash", key)
	}

	count := 0
	for field, value := range entries {
		if strings.EqualFold(cmd[0], "hsetnx") {
			if hash[field] == nil {
				hash[field] = value
				count += 1
			}
			continue
		}
		hash[field] = value
		count += 1
	}
	server.SetValue(ctx, key, hash)

	return []byte(fmt.Sprintf(":%d\r\n\r\n", count)), nil
}

func handleHGET(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	if len(cmd) < 3 {
		return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
	}

	key := cmd[1]
	fields := cmd[2:]

	if !server.KeyExists(key) {
		return []byte("$-1\r\n\r\n"), nil
	}

	_, err := server.KeyRLock(ctx, key)
	if err != nil {
		return nil, err
	}
	defer server.KeyRUnlock(key)

	hash, ok := server.GetValue(key).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value at %s is not a hash", key)
	}

	var value interface{}

	res := fmt.Sprintf("*%d\r\n", len(fields))
	for _, field := range fields {
		value = hash[field]
		if value == nil {
			res += "$-1\r\n"
			continue
		}
		if s, ok := value.(string); ok {
			res += fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
			continue
		}
		if d, ok := value.(int); ok {
			res += fmt.Sprintf(":%d\r\n", d)
			continue
		}
		if f, ok := value.(float64); ok {
			fs := strconv.FormatFloat(f, 'f', -1, 64)
			res += fmt.Sprintf("$%d\r\n%s\r\n", len(fs), fs)
			continue
		}
		res += fmt.Sprintf("$-1\r\n")
	}
	res += "\r\n"

	return []byte(res), nil
}

func handleHSTRLEN(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	if len(cmd) < 3 {
		return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
	}

	key := cmd[1]
	fields := cmd[2:]

	if !server.KeyExists(key) {
		return []byte("$-1\r\n\r\n"), nil
	}

	_, err := server.KeyRLock(ctx, key)
	if err != nil {
		return nil, err
	}
	defer server.KeyRUnlock(key)

	hash, ok := server.GetValue(key).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value at %s is not a hash", key)
	}

	var value interface{}

	res := fmt.Sprintf("*%d\r\n", len(fields))
	for _, field := range fields {
		value = hash[field]
		if value == nil {
			res += ":0\r\n"
			continue
		}
		if s, ok := value.(string); ok {
			res += fmt.Sprintf(":%d\r\n", len(s))
			continue
		}
		if f, ok := value.(float64); ok {
			fs := strconv.FormatFloat(f, 'f', -1, 64)
			res += fmt.Sprintf(":%d\r\n", len(fs))
			continue
		}
		if d, ok := value.(int); ok {
			res += fmt.Sprintf(":%d\r\n", d)
			continue
		}
		res += ":0\r\n"
	}
	res += "\r\n"

	return []byte(res), nil
}

func handleHVALS(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	if len(cmd) != 2 {
		return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
	}
	key := cmd[1]

	if !server.KeyExists(key) {
		return []byte("*0\r\n\r\n"), nil
	}

	_, err := server.KeyRLock(ctx, key)
	if err != nil {
		return nil, err
	}
	defer server.KeyRUnlock(key)

	hash, ok := server.GetValue(key).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value at %s is not a hash", key)
	}

	res := fmt.Sprintf("*%d\r\n", len(hash))
	for _, val := range hash {
		if s, ok := val.(string); ok {
			res += fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
			continue
		}
		if f, ok := val.(float64); ok {
			fs := strconv.FormatFloat(f, 'f', -1, 64)
			res += fmt.Sprintf("$%d\r\n%s\r\n", len(fs), fs)
			continue
		}
		if d, ok := val.(int); ok {
			res += fmt.Sprintf(":%d\r\n", d)
		}
	}
	res += "\r\n"

	return []byte(res), nil
}

func handleHRANDFIELD(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	if len(cmd) < 2 || len(cmd) > 4 {
		return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
	}

	key := cmd[1]

	count := 1
	if len(cmd) >= 3 {
		c, err := strconv.Atoi(cmd[2])
		if err != nil {
			return nil, errors.New("count must be an integer")
		}
		if c == 0 {
			return []byte("*0\r\n\r\n"), nil
		}
		count = c
	}

	withvalues := false
	if len(cmd) == 4 {
		if strings.EqualFold(cmd[3], "withvalues") {
			withvalues = true
		} else {
			return nil, errors.New("result modifier must be withvalues")
		}
	}

	if !server.KeyExists(key) {
		return []byte("*0\r\n\r\n"), nil
	}

	if _, err := server.KeyRLock(ctx, key); err != nil {
		return nil, err
	}
	defer server.KeyRUnlock(key)

	hash, ok := server.GetValue(key).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value at %s is not a hash", key)
	}

	// If count is the >= hash length, then return the entire hash
	if count >= len(hash) {
		res := fmt.Sprintf("*%d\r\n", len(hash))
		if withvalues {
			res = fmt.Sprintf("*%d\r\n", len(hash)*2)
		}
		for field, value := range hash {
			res += fmt.Sprintf("$%d\r\n%s\r\n", len(field), field)
			if withvalues {
				if s, ok := value.(string); ok {
					res += fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
					continue
				}
				if f, ok := value.(float64); ok {
					fs := strconv.FormatFloat(f, 'f', -1, 64)
					res += fmt.Sprintf("$%d\r\n%s\r\n", len(fs), fs)
					continue
				}
				if d, ok := value.(int); ok {
					res += fmt.Sprintf(":%d\r\n", d)
					continue
				}
			}
		}
		res += "\r\n"
		return []byte(res), nil
	}

	// Get all the fields
	var fields []string
	for field, _ := range hash {
		fields = append(fields, field)
	}

	// Pluck fields and return them
	var pluckedFields []string
	var n int
	for i := 0; i < utils.AbsInt(count); i++ {
		n = rand.Intn(len(fields))
		pluckedFields = append(pluckedFields, fields[n])
		// If count is positive, remove the current field from list of fields
		if count > 0 {
			fields = slices.DeleteFunc(fields, func(s string) bool {
				return s == fields[n]
			})
		}
	}

	res := fmt.Sprintf("*%d\r\n", len(pluckedFields))
	if withvalues {
		res = fmt.Sprintf("*%d\r\n", len(pluckedFields)*2)
	}
	for _, field := range pluckedFields {
		res += fmt.Sprintf("$%d\r\n%s\r\n", len(field), field)
		if withvalues {
			if s, ok := hash[field].(string); ok {
				res += fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
				continue
			}
			if f, ok := hash[field].(float64); ok {
				fs := strconv.FormatFloat(f, 'f', -1, 64)
				res += fmt.Sprintf("$%d\r\n%s\r\n", len(fs), fs)
				continue
			}
			if d, ok := hash[field].(int); ok {
				res += fmt.Sprintf(":%d\r\n", d)
				continue
			}
		}
	}
	res += "\r\n"

	return []byte(res), nil
}

func handleHLEN(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	if len(cmd) != 2 {
		return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
	}

	key := cmd[1]

	if !server.KeyExists(key) {
		return []byte(":0\r\n\r\n"), nil
	}

	if _, err := server.KeyRLock(ctx, key); err != nil {
		return nil, err
	}
	defer server.KeyRUnlock(key)

	hash, ok := server.GetValue(key).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value at %s is not a hash", key)
	}

	return []byte(fmt.Sprintf(":%d\r\n\r\n", len(hash))), nil
}

func handleHKEYS(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	if len(cmd) != 2 {
		return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
	}

	key := cmd[1]

	if !server.KeyExists(key) {
		return []byte("*0\r\n\r\n"), nil
	}

	if _, err := server.KeyRLock(ctx, key); err != nil {
		return nil, err
	}
	defer server.KeyRUnlock(key)

	hash, ok := server.GetValue(key).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value at %s is not a hash", key)
	}

	res := fmt.Sprintf("*%d\r\n", len(hash))
	for field, _ := range hash {
		res += fmt.Sprintf("$%d\r\n%s\r\n", len(field), field)
	}
	res += "\r\n"

	return []byte(res), nil
}

func handleINCRBY(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	if len(cmd) != 4 {
		return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
	}

	key := cmd[1]
	field := cmd[2]

	var intIncrement int
	var floatIncrement float64
	if strings.EqualFold(cmd[0], "hincrbyfloat") {
		f, err := strconv.ParseFloat(cmd[3], 64)
		if err != nil {
			return nil, errors.New("increment must be a float")
		}
		floatIncrement = f
	} else {
		i, err := strconv.Atoi(cmd[3])
		if err != nil {
			return nil, errors.New("increment must be an integer")
		}
		intIncrement = i
	}

	if !server.KeyExists(key) {
		if _, err := server.CreateKeyAndLock(ctx, key); err != nil {
			return nil, err
		}
		defer server.KeyUnlock(key)
		hash := make(map[string]interface{})
		if strings.EqualFold(cmd[0], "hincrbyfloat") {
			hash[field] = floatIncrement
			server.SetValue(ctx, key, hash)
			return []byte(fmt.Sprintf("+%s\r\n\r\n", strconv.FormatFloat(floatIncrement, 'f', -1, 64))), nil
		} else {
			hash[field] = intIncrement
			server.SetValue(ctx, key, hash)
			return []byte(fmt.Sprintf(":%d\r\n\r\n", intIncrement)), nil
		}
	}

	if _, err := server.KeyLock(ctx, key); err != nil {
		return nil, err
	}
	defer server.KeyUnlock(key)

	hash, ok := server.GetValue(key).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value at %s is not a hash", key)
	}

	if hash[field] == nil {
		hash[field] = 0
	}

	switch hash[field].(type) {
	default:
		return nil, errors.New("hash value is not an integer")
	case int:
		i, _ := hash[field].(int)
		if strings.EqualFold(cmd[0], "hincrbyfloat") {
			hash[field] = float64(i) + floatIncrement
		} else {
			hash[field] = i + intIncrement
		}
	case float64:
		f, _ := hash[field].(float64)
		if strings.EqualFold(cmd[0], "hincrbyfloat") {
			hash[field] = f + floatIncrement
		} else {
			hash[field] = f + float64(intIncrement)
		}
	}

	server.SetValue(ctx, key, hash)

	if f, ok := hash[field].(float64); ok {
		return []byte(fmt.Sprintf("+%s\r\n\r\n", strconv.FormatFloat(f, 'f', -1, 64))), nil
	}

	i, _ := hash[field].(int)
	return []byte(fmt.Sprintf(":%d\r\n\r\n", i)), nil
}

func handleHGETALL(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	if len(cmd) != 2 {
		return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
	}

	key := cmd[1]

	if !server.KeyExists(key) {
		return []byte("*0\r\n\r\n"), nil
	}

	if _, err := server.KeyRLock(ctx, key); err != nil {
		return nil, err
	}
	defer server.KeyRUnlock(key)

	hash, ok := server.GetValue(key).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value at %s is not a hash", key)
	}

	res := fmt.Sprintf("*%d\r\n", len(hash)*2)
	for field, value := range hash {
		res += fmt.Sprintf("$%d\r\n%s\r\n", len(field), field)
		if s, ok := value.(string); ok {
			res += fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
		}
		if f, ok := value.(float64); ok {
			fs := strconv.FormatFloat(f, 'f', -1, 64)
			res += fmt.Sprintf("$%d\r\n%s\r\n", len(fs), fs)
		}
		if d, ok := value.(int); ok {
			res += fmt.Sprintf(":%d\r\n", d)
		}
	}
	res += "\r\n"

	return []byte(res), nil
}

func handleHEXISTS(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	if len(cmd) != 3 {
		return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
	}

	key := cmd[1]
	field := cmd[2]

	if !server.KeyExists(key) {
		return []byte(":0\r\n\r\n"), nil
	}

	if _, err := server.KeyRLock(ctx, key); err != nil {
		return nil, err
	}
	defer server.KeyRUnlock(key)

	hash, ok := server.GetValue(key).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value at %s is not a hash", key)
	}

	if hash[field] != nil {
		return []byte(":1\r\n\r\n"), nil
	}

	return []byte(":0\r\n\r\n"), nil
}

func handleHDEL(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	if len(cmd) < 3 {
		return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
	}

	key := cmd[1]
	fields := cmd[2:]

	if !server.KeyExists(key) {
		return []byte(":0\r\n\r\n"), nil
	}

	if _, err := server.KeyLock(ctx, key); err != nil {
		return nil, err
	}
	defer server.KeyUnlock(key)

	hash, ok := server.GetValue(key).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("value at %s is not a hash", key)
	}

	count := 0

	for _, field := range fields {
		if hash[field] != nil {
			delete(hash, field)
			count += 1
		}
	}

	server.SetValue(ctx, key, hash)

	return []byte(fmt.Sprintf(":%d\r\n\r\n", count)), nil
}

func NewModule() Plugin {
	SetModule := Plugin{
		name: "HashCommands",
		commands: []utils.Command{
			{
				Command:     "hset",
				Categories:  []string{utils.HashCategory, utils.WriteCategory, utils.FastCategory},
				Description: `(HSET key field value [field value ...]) Set update each field of the hash with the corresponding value`,
				Sync:        true,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) < 4 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return cmd[1:2], nil
				},
				HandlerFunc: handleHSET,
			},
			{
				Command:     "hsetnx",
				Categories:  []string{utils.HashCategory, utils.WriteCategory, utils.FastCategory},
				Description: `(HSETNX key field value [field value ...]) Set hash field value only if the field does not exist`,
				Sync:        true,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) < 4 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return cmd[1:2], nil
				},
				HandlerFunc: handleHSET,
			},
			{
				Command:     "hget",
				Categories:  []string{utils.HashCategory, utils.ReadCategory, utils.FastCategory},
				Description: `(HGET key field [field ...]) Retrieve the of each of the listed fields from the hash`,
				Sync:        false,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) < 3 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return cmd[1:2], nil
				},
				HandlerFunc: handleHGET,
			},
			{
				Command:    "hstrlen",
				Categories: []string{utils.HashCategory, utils.ReadCategory, utils.FastCategory},
				Description: `(HSTRLEN key field [field ...]) 
Return the string length of the values stored at the specified fields. 0 if the value does not exist`,
				Sync: false,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) < 3 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return cmd[1:2], nil
				},
				HandlerFunc: handleHSTRLEN,
			},
			{
				Command:     "hvals",
				Categories:  []string{utils.HashCategory, utils.ReadCategory, utils.SlowCategory},
				Description: `(HVALS key) Returns all the values of the hash at key.`,
				Sync:        false,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) != 2 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return cmd[1:], nil
				},
				HandlerFunc: handleHVALS,
			},
			{
				Command:     "hrandfield",
				Categories:  []string{utils.HashCategory, utils.ReadCategory, utils.SlowCategory},
				Description: `(HRANDFIELD key [count [WITHVALUES]]) Returns one or more random fields from the hash`,
				Sync:        false,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) < 2 || len(cmd) > 4 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					if len(cmd) == 2 {
						return cmd[1:], nil
					}
					return cmd[1:2], nil
				},
				HandlerFunc: handleHRANDFIELD,
			},
			{
				Command:     "hlen",
				Categories:  []string{utils.HashCategory, utils.ReadCategory, utils.FastCategory},
				Description: `(HLEN key) Returns the number of fields in the hash`,
				Sync:        false,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) != 2 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return cmd[1:], nil
				},
				HandlerFunc: handleHLEN,
			},
			{
				Command:     "hkeys",
				Categories:  []string{utils.HashCategory, utils.ReadCategory, utils.SlowCategory},
				Description: `(HKEYS key) Returns all the fields in a hash`,
				Sync:        false,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) != 2 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return cmd[1:], nil
				},
				HandlerFunc: handleHKEYS,
			},
			{
				Command:     "hincrbyfloat",
				Categories:  []string{utils.HashCategory, utils.WriteCategory, utils.FastCategory},
				Description: `(HINCRBYFLOAT key field increment) Increment the hash value by the float increment`,
				Sync:        true,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) != 4 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return cmd[1:2], nil
				},
				HandlerFunc: handleINCRBY,
			},
			{
				Command:     "hincrby",
				Categories:  []string{utils.HashCategory, utils.WriteCategory, utils.FastCategory},
				Description: `(HINCRBY key field increment) Increment the hash value by the integer increment`,
				Sync:        true,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) != 4 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return cmd[1:2], nil
				},
				HandlerFunc: handleINCRBY,
			},
			{
				Command:     "hgetall",
				Categories:  []string{utils.HashCategory, utils.ReadCategory, utils.SlowCategory},
				Description: `(HGETALL key) Get all fields and values of a hash`,
				Sync:        false,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) != 2 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return cmd[1:], nil
				},
				HandlerFunc: handleHGETALL,
			},
			{
				Command:     "hexists",
				Categories:  []string{utils.HashCategory, utils.ReadCategory, utils.FastCategory},
				Description: `(HEXISTS key field) Returns if field is an existing field in the hash`,
				Sync:        false,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) != 3 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return cmd[1:2], nil
				},
				HandlerFunc: handleHEXISTS,
			},
			{
				Command:     "hdel",
				Categories:  []string{utils.HashCategory, utils.ReadCategory, utils.FastCategory},
				Description: `(HDEL key field [field ...]) Deletes the specified fields from the hash`,
				Sync:        true,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) < 3 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return cmd[1:2], nil
				},
				HandlerFunc: handleHDEL,
			},
		},
		description: "Handle HASH commands",
	}

	return SetModule
}
