package dnsserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/miekg/dns"
	"github.com/redis/go-redis/v9"
)

const recordKeyPrefix = "dns:record:"

type RecordStore interface {
	Get(context.Context, string) (string, error)
	SetNX(context.Context, string, string) (bool, error)
}

type RecordAdminStore interface {
	RecordStore
	ListRecords(context.Context) ([]Record, error)
	UpsertRecord(context.Context, Record) error
	DeleteRecord(context.Context, string, uint16) error
	Ping(context.Context) error
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) RedisStore {
	return RedisStore{client: client}
}

func (s RedisStore) Get(ctx context.Context, key string) (string, error) {
	return s.client.Get(ctx, key).Result()
}

func (s RedisStore) SetNX(ctx context.Context, key, value string) (bool, error) {
	return s.client.SetNX(ctx, key, value, 0).Result()
}

func (s RedisStore) ListRecords(ctx context.Context) ([]Record, error) {
	var records []Record
	var cursor uint64
	for {
		keys, next, err := s.client.Scan(ctx, cursor, recordKeyPrefix+"*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("list DNS records: %w", err)
		}
		for _, key := range keys {
			if len(key) >= len(":_name") && key[len(key)-len(":_name"):] == ":_name" {
				continue
			}
			value, err := s.client.Get(ctx, key).Result()
			if err != nil {
				return nil, fmt.Errorf("read DNS record %q: %w", key, err)
			}
			record, err := decodeRecord(value)
			if err != nil {
				return nil, fmt.Errorf("decode DNS record %q: %w", key, err)
			}
			records = append(records, record)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Name != records[j].Name {
			return records[i].Name < records[j].Name
		}
		return records[i].Type < records[j].Type
	})
	return records, nil
}

func (s RedisStore) UpsertRecord(ctx context.Context, record Record) error {
	if err := ValidateRecord(record); err != nil {
		return err
	}
	record.Name = NormalizeName(record.Name)
	value, err := encodeRecord(record)
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, nameKey(record.Name), "1", 0)
	pipe.Set(ctx, RecordKey(record.Name, record.Type), value, 0)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("upsert DNS record: %w", err)
	}
	return nil
}

func (s RedisStore) DeleteRecord(ctx context.Context, name string, recordType uint16) error {
	name = NormalizeName(name)
	if !supportedType(recordType) {
		return fmt.Errorf("unsupported record type %d", recordType)
	}
	deleted, err := s.client.Del(ctx, RecordKey(name, recordType)).Result()
	if err != nil {
		return fmt.Errorf("delete DNS record: %w", err)
	}
	if deleted == 0 {
		return redis.Nil
	}
	keys, err := s.client.Keys(ctx, recordKeyPrefix+name+":*").Result()
	if err != nil {
		return fmt.Errorf("check remaining DNS records: %w", err)
	}
	if len(keys) == 1 && keys[0] == nameKey(name) {
		if _, err := s.client.Del(ctx, nameKey(name)).Result(); err != nil {
			return fmt.Errorf("delete DNS name marker: %w", err)
		}
	}
	return nil
}

func (s RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func RecordKey(name string, recordType uint16) string {
	return recordKeyPrefix + NormalizeName(name) + ":" + dns.TypeToString[recordType]
}

func nameKey(name string) string {
	return recordKeyPrefix + NormalizeName(name) + ":_name"
}

func encodeRecord(record Record) (string, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return "", fmt.Errorf("encode DNS record: %w", err)
	}
	return string(data), nil
}

func decodeRecord(value string) (Record, error) {
	var record Record
	if err := json.Unmarshal([]byte(value), &record); err != nil {
		return Record{}, fmt.Errorf("decode DNS record: %w", err)
	}
	record.Name = NormalizeName(record.Name)
	if err := ValidateRecord(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func SeedRecords(ctx context.Context, records []Record, store RecordStore) error {
	for _, record := range records {
		if err := ValidateRecord(record); err != nil {
			return fmt.Errorf("validate record %q: %w", record.Name, err)
		}
		record.Name = NormalizeName(record.Name)
		if _, err := store.SetNX(ctx, nameKey(record.Name), "1"); err != nil {
			return fmt.Errorf("seed %s: %w", nameKey(record.Name), err)
		}
		value, err := encodeRecord(record)
		if err != nil {
			return err
		}
		if _, err := store.SetNX(ctx, RecordKey(record.Name, record.Type), value); err != nil {
			return fmt.Errorf("seed %s: %w", RecordKey(record.Name, record.Type), err)
		}
	}
	return nil
}

func recordNameExists(ctx context.Context, store RecordStore, name string) (bool, error) {
	_, err := store.Get(ctx, nameKey(name))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	return false, err
}

func lookupRecord(ctx context.Context, store RecordStore, name string, recordType uint16) (Record, error) {
	value, err := store.Get(ctx, RecordKey(name, recordType))
	if err != nil {
		return Record{}, err
	}
	return decodeRecord(value)
}

func InitRedisClientWithConfig(config Config) (*redis.Client, context.Context) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
		Protocol: 2,
	})
	return client, context.Background()
}

func InitRedisClient() (*redis.Client, context.Context) {
	return InitRedisClientWithConfig(DefaultConfig())
}
