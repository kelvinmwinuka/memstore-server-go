package etc

import (
	"context"
	"errors"
	"fmt"
	"github.com/kelvinmwinuka/memstore/src/utils"
	"net"
	"time"
)

type KeyObject struct {
	value  interface{}
	locked bool
}

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

func handleSet(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	switch x := len(cmd); {
	default:
		return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
	case x == 3:
		key := cmd[1]

		if !server.KeyExists(key) {
			_, err := server.CreateKeyAndLock(ctx, key)
			if err != nil {
				return nil, err
			}
			server.SetValue(ctx, key, utils.AdaptType(cmd[2]))
			server.KeyUnlock(key)
			return []byte(utils.OK_RESPONSE), nil
		}

		if _, err := server.KeyLock(ctx, key); err != nil {
			return nil, err
		}

		server.SetValue(ctx, key, utils.AdaptType(cmd[2]))
		server.KeyUnlock(key)
		return []byte(utils.OK_RESPONSE), nil
	}
}

func handleSetNX(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	switch x := len(cmd); {
	default:
		return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
	case x == 3:
		key := cmd[1]
		if server.KeyExists(key) {
			return nil, fmt.Errorf("key %s already exists", cmd[1])
		}
		// TODO: Retry CreateKeyAndLock until we manage to obtain the key
		_, err := server.CreateKeyAndLock(ctx, key)
		if err != nil {
			return nil, err
		}
		server.SetValue(ctx, key, utils.AdaptType(cmd[2]))
		server.KeyUnlock(key)
	}
	return []byte(utils.OK_RESPONSE), nil
}

func handleMSet(ctx context.Context, cmd []string, server utils.Server, conn *net.Conn) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()

	// Check if key/value pairs are complete
	if len(cmd[1:])%2 != 0 {
		return nil, errors.New("each key must have a matching value")
	}

	entries := make(map[string]KeyObject)

	// Release all acquired key locks
	defer func() {
		for k, v := range entries {
			if v.locked {
				server.KeyUnlock(k)
				entries[k] = KeyObject{
					value:  v.value,
					locked: false,
				}
			}
		}
	}()

	// Extract all the key/value pairs
	for i, key := range cmd[1:] {
		if i%2 == 0 {
			entries[key] = KeyObject{
				value:  utils.AdaptType(cmd[1:][i+1]),
				locked: false,
			}
		}
	}

	// Acquire all the locks for each key first
	// If any key cannot be acquired, abandon transaction and release all currently held keys
	for k, v := range entries {
		if server.KeyExists(k) {
			if _, err := server.KeyLock(ctx, k); err != nil {
				return nil, err
			}
			entries[k] = KeyObject{value: v.value, locked: true}
			continue
		}
		if _, err := server.CreateKeyAndLock(ctx, k); err != nil {
			return nil, err
		}
		entries[k] = KeyObject{value: v.value, locked: true}
	}

	// Set all the values
	for k, v := range entries {
		server.SetValue(ctx, k, v.value)
	}

	return []byte(utils.OK_RESPONSE), nil
}

func NewModule() Plugin {
	SetModule := Plugin{
		name: "OtherCommands",
		commands: []utils.Command{
			{
				Command:     "set",
				Categories:  []string{utils.WriteCategory, utils.SlowCategory},
				Description: "(SET key value) Set the value of a key, considering the value's type.",
				Sync:        true,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) != 3 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return []string{cmd[1]}, nil
				},
				HandlerFunc: handleSet,
			},
			{
				Command:     "setnx",
				Categories:  []string{utils.WriteCategory, utils.SlowCategory},
				Description: "(SETNX key value) Set the key/value only if the key doesn't exist.",
				Sync:        true,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd) != 3 {
						return nil, errors.New(utils.WRONG_ARGS_RESPONSE)
					}
					return []string{cmd[1]}, nil
				},
				HandlerFunc: handleSetNX,
			},
			{
				Command:     "mset",
				Categories:  []string{utils.WriteCategory, utils.SlowCategory},
				Description: "(MSET key value [key value ...]) Automatically etc or modify multiple key/value pairs.",
				Sync:        true,
				KeyExtractionFunc: func(cmd []string) ([]string, error) {
					if len(cmd[1:])%2 != 0 {
						return nil, errors.New("each key must be paired with a value")
					}
					var keys []string
					for i, key := range cmd[1:] {
						if i%2 == 0 {
							keys = append(keys, key)
						}
					}
					return keys, nil
				},
				HandlerFunc: handleMSet,
			},
		},
		description: "Handle basic SET commands",
	}

	return SetModule
}
