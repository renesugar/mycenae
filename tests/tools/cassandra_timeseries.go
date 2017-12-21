package tools

import (
	"fmt"
	"hash/crc32"
	"log"
	"sort"
	"time"
	"github.com/gocql/gocql"
)

type cassTs struct {
	cql *gocql.Session
}

func (ts *cassTs) init(cql *gocql.Session) {
	ts.cql = cql
}

type TableProperties struct {
	Bloom_filter_fp_chance      float64
	Caching                     map[string]string
	Comment                     string
	Compaction                  map[string]string
	Compression                 map[string]string
	Dclocal_read_repair_chance  float64
	Default_time_to_live        int
	Gc_grace_seconds            int
	Max_index_interval          int
	Memtable_flush_period_in_ms int
	Min_index_interval          int
	Read_repair_chance          float64
	Speculative_retry           string
}

type KeyspaceAttributes struct {
	Name                    string
	Replication_factor      int
	Datacenter              string
	Ks_ttl                  int
	Ks_tuuid                bool
	Contact                 string
	Replication_factor_meta string
}

type KeyspaceProperties struct {
	Keyspace_name  string
	Durable_writes bool
	Replication    map[string]string
}

const (
	cqlKeyspaceTables = `SELECT table_name FROM system_schema.tables WHERE keyspace_name = ?`
	cqlExists = `SELECT keyspace_name FROM system_schema.keyspaces WHERE keyspace_name = ?`
	cqlExistsInformation = `SELECT name, datacenter, contact, replication_factor, ks_ttl, ks_tuuid FROM mycenae.ts_keyspace WHERE key = ?`
	cqlTableProperties = `SELECT bloom_filter_fp_chance, caching, comment, compaction, compression, dclocal_read_repair_chance, default_time_to_live, gc_grace_seconds, max_index_interval, memtable_flush_period_in_ms, min_index_interval, read_repair_chance, speculative_retry from system_schema.tables  where keyspace_name = ? and table_name = ?`
	cqlKeyspaceProperties = `SELECT keyspace_name, durable_writes, replication from system_schema.keyspaces where keyspace_name = ?`
	cqlDropKS = `DROP KEYSPACE %v`
	cqlSelectKS = `SELECT name, replication_factor, datacenter, ks_ttl, ks_tuuid, contact, replication_factor_meta FROM mycenae.ts_keyspace WHERE key = ?`
	cqlDeleteKS = `DELETE FROM mycenae.ts_keyspace WHERE key = '%v'`
	cqlInsertKS = `INSERT INTO mycenae.ts_keyspace (key, name , datacenter , replication_factor, ks_ttl, ks_tuuid) VALUES ('%v', '%v', 'dc_gt_a1', 1, 90, false)`
)
//
//func (ts *cassTs) countByValueInColumn(column string, table string, namespace string, funcName string, value string) (int, gobol.Error) {
//
//	it := ts.cql.Query(fmt.Sprintf("SELECT %s FROM %s.%s", column, namespace, table)).Iter()
//
//	var count int
//	var scanned string
//	for it.Scan(&scanned) {
//		if value == scanned {
//			count++;
//		}
//	}
//
//	if err := it.Close(); err != nil {
//		log.Println(err)
//	}
//
//	return count, nil
//}

func (ts *cassTs) GetValueFromIDSTAMP(keyspace, id string) (nValue float64) {
	if err := ts.cql.Query(fmt.Sprintf(`SELECT value FROM %s.ts_number_stamp WHERE id=?`, keyspace),
		id,
	).Scan(&nValue); err != nil && err != gocql.ErrNotFound {
		log.Println(err)
	}
	return
}

func (ts *cassTs) GetValueFromDateSTAMP(keyspace, id string, date time.Time) (nValue float64) {
	if err := ts.cql.Query(fmt.Sprintf(`SELECT value FROM %s.ts_number_stamp WHERE id=? AND date=?`, keyspace),
		id,
		date,
	).Scan(&nValue); err != nil && err != gocql.ErrNotFound {
		log.Println(err)
	}
	return
}

func (ts *cassTs) GetTextFromDateSTAMP(keyspace, id string, date time.Time) (nValue string) {
	if err := ts.cql.Query(fmt.Sprintf(`SELECT value FROM %s.ts_text_stamp WHERE id=? AND date=?`, keyspace),
		id,
		date.Truncate(time.Second),
	).Scan(&nValue); err != nil && err != gocql.ErrNotFound {
		log.Println(err)
	}
	return
}

func (ts *cassTs) CountValueFromIDSTAMP(keyspace, id string) (int) {
	it := ts.cql.Query(fmt.Sprintf("SELECT id FROM %s.ts_number_stamp WHERE id=?", keyspace),id,).Iter()

	var count int
	var scanned string
	for it.Scan(&scanned) {
		if id == scanned {
			count++;
		}
	}
	if err := it.Close(); err != nil {
		log.Println(err)
	}
	return count
}

//func (ts *cassTs) CountValueFromIDSTAMP(keyspace, id string) (count int) {
//	if err := ts.cql.Query(fmt.Sprintf(`SELECT count(*) FROM %s.ts_number_stamp WHERE id=?`, keyspace),
//		id,
//	).Scan(&count); err != nil {
//		log.Println(err)
//	}
//	return
//}


func (ts *cassTs) CountTextFromIDSTAMP(keyspace, id string) (int) {
	it := ts.cql.Query(fmt.Sprintf("SELECT id FROM %s.ts_text_stamp WHERE id=?", keyspace),id,).Iter()

	var count int
	var scanned string
	for it.Scan(&scanned) {
		if id == scanned {
			count++;
		}
	}
	if err := it.Close(); err != nil {
		log.Println(err)
	}
	return count
}

//func (ts *cassTs) CountTextFromIDSTAMP(keyspace, id string) (count int) {
//	if err := ts.cql.Query(fmt.Sprintf(`SELECT count(*) FROM %s.ts_text_stamp WHERE id=?`, keyspace),
//		id,
//	).Scan(&count); err != nil {
//		log.Println(err)
//	}
//	return
//}

//func (ts *cassTs) GetValueTTLDaysFromDateSTAMP(id string, date time.Time) (days float64) {
//	var seconds int
//	if err := ts.cql.Query(`SELECT ttl(value) FROM ts_number_stamp WHERE id=? AND date=?`,
//		id,
//		date,
//	).Scan(&seconds); err != nil && err != gocql.ErrNotFound {
//		log.Println(err)
//	}
//	days = math.Ceil(float64(seconds) / 60 / 60 / 24)
//	return
//}

func (ts *cassTs) GetValueFromTwoDatesSTAMP(keyspace, id string, dateBeforeRequest time.Time, dateAfterRequest time.Time) (nValue float64) {
	if err := ts.cql.Query(fmt.Sprintf(`SELECT value FROM %s.ts_number_stamp WHERE id=? AND date >= ? AND date <= ?`, keyspace),
		id,
		dateBeforeRequest,
		dateAfterRequest,
	).Scan(&nValue); err != nil && err != gocql.ErrNotFound {
		log.Println(err)
	}
	return
}

func (ts *cassTs) GetTextFromTwoDatesSTAMP(keyspace, id string, dateBeforeRequest time.Time, dateAfterRequest time.Time) (nValue string) {
	if err := ts.cql.Query(fmt.Sprintf(`SELECT value FROM %s.ts_text_stamp WHERE id=? AND date >= ? AND date <= ?`, keyspace),
		id,
		dateBeforeRequest.Truncate(time.Second),
		dateAfterRequest.Truncate(time.Second),
	).Scan(&nValue); err != nil && err != gocql.ErrNotFound {
		log.Println(err)
	}
	return
}

func (ts *cassTs) CountValueFromIDAndDateSTAMP(keyspace, id string, date time.Time) (int) {
	it := ts.cql.Query(fmt.Sprintf("SELECT id FROM %s.ts_number_stamp WHERE id=? AND date=?", keyspace), id, date.Truncate(time.Second)).Iter()

	var count int
	var scanned string
	for it.Scan(&scanned) {
		if id == scanned {
			count++;
		}
	}
	if err := it.Close(); err != nil {
		log.Println(err)
	}
	return count
}

//func (ts *cassTs) CountValueFromIDAndDateSTAMP(keyspace, id string, date time.Time) (count int) {
//	if err := ts.cql.Query(fmt.Sprintf(`SELECT count(*) FROM %s.ts_number_stamp WHERE id=? AND date=?`, keyspace),
//		id,
//		date,
//	).Scan(&count); err != nil {
//		log.Println(err)
//	}
//	return
//}

func (ts *cassTs) CountTextFromIDAndDateSTAMP(keyspace, id string, date time.Time) (int) {
	it := ts.cql.Query(fmt.Sprintf("SELECT id FROM %s.ts_text_stamp WHERE id=? AND date=?", keyspace), id, date.Truncate(time.Second)).Iter()

	var count int
	var scanned string
	for it.Scan(&scanned) {
		if id == scanned {
			count++;
		}
	}
	if err := it.Close(); err != nil {
		log.Println(err)
	}
	return count
}

//func (ts *cassTs) CountTextFromIDAndDateSTAMP(keyspace, id string, date time.Time) (count int) {
//	if err := ts.cql.Query(fmt.Sprintf(`SELECT count(*) FROM %s.ts_text_stamp WHERE id=? AND date=?`, keyspace),
//		id,
//		date.Truncate(time.Second),
//	).Scan(&count); err != nil {
//		log.Println(err)
//	}
//	return
//}

func (ts *cassTs) GetHashFromMetricAndTags(metric string, tags map[string]string) string {
	h := crc32.NewIEEE()
	h.Write([]byte(metric))
	mk := []string{}

	for k := range tags {
		if k != "ksid" && k != "ttl" && k != "tuuid" {
			mk = append(mk, k)
		}
	}

	sort.Strings(mk)

	for _, k := range mk {
		h.Write([]byte(k))
		h.Write([]byte(tags[k]))
	}

	return fmt.Sprint(h.Sum32())
}

func (ts *cassTs) GetTextHashFromMetricAndTags(metric string, tags map[string]string) string {
	h := crc32.NewIEEE()
	h.Write([]byte(metric))
	mk := []string{}

	for k := range tags {
		if k != "ksid" && k != "ttl" && k != "tuuid" {
			mk = append(mk, k)
		}
	}

	sort.Strings(mk)

	for _, k := range mk {
		h.Write([]byte(k))
		h.Write([]byte(tags[k]))
	}

	return fmt.Sprint("T", h.Sum32())
}

func (ts *cassTs) CountKeyspaces() (int) {
	it := ts.cql.Query("SELECT id FROM system_schema.keyspaces").Iter()

	var count int
	var scanned string
	for it.Scan(&scanned) {
			count++;
	}
	if err := it.Close(); err != nil {
		log.Println(err)
	}
	return count
}

//func (ts *cassTs) CountKeyspaces() (count int) {
//	if err := ts.cql.Query(cqlCountKeyspaces).Scan(&count); err != nil {
//		log.Println(err)
//	}
//	return
//}

func (ts *cassTs) CountTsKeyspaces() (int) {
	it := ts.cql.Query("SELECT id FROM mycenae.ts_keyspace").Iter()

	var count int
	var scanned string
	for it.Scan(&scanned) {
		count++;
	}
	if err := it.Close(); err != nil {
		log.Println(err)
	}
	return count
}

//func (ts *cassTs) CountTsKeyspaces() (count int) {
//	if err := ts.cql.Query(cqlCountTsKeyspaces).Scan(&count); err != nil {
//		log.Println(err)
//	}
//	return
//}

func (ts *cassTs) CountTsKeyspaceByKsid(keyspace string) (int) {
	it := ts.cql.Query("SELECT key FROM mycenae.ts_keyspace WHERE key=?",keyspace,).Iter()

	var count int
	var scanned string
	for it.Scan(&scanned) {
		if keyspace == scanned {
			count++;
		}
	}
	if err := it.Close(); err != nil {
		log.Println(err)
	}
	return count
}

//func (ts *cassTs) CountTsKeyspaceByName(name string) int {
//	var count1, count2 int
//	if err := ts.cql.Query(`SELECT count(*) FROM mycenae.ts_keyspace WHERE name = ? AND token(key) < 0;`, name).Scan(&count1); err != nil {
//		log.Println(err)
//	}
//	if err := ts.cql.Query(`SELECT count(*) FROM mycenae.ts_keyspace WHERE name = ? AND token(key) >= 0;`, name).Scan(&count2); err != nil {
//		log.Println(err)
//	}
//	return count1 + count2
//}

func (ts *cassTs) CountKeyspacesNoCassWarning() int {
	var count1, count2 int
	if err := ts.cql.Query(`SELECT count(*) FROM system_schema.keyspaces WHERE token(keyspace_name) < 0;`).Scan(&count1); err != nil {
		log.Println(err)
	}
	if err := ts.cql.Query(`SELECT count(*) FROM system_schema.keyspaces WHERE token(keyspace_name) >= 0;`).Scan(&count2); err != nil {
		log.Println(err)
	}
	return count1 + count2
}

func (ts *cassTs) CountTsKeyspacesNoCassWarning() int {
	var count1, count2 int
	if err := ts.cql.Query(`SELECT count(*) FROM mycenae.ts_keyspace WHERE token(key) < 0;`).Scan(&count1); err != nil {
		log.Println(err)
	}
	if err := ts.cql.Query(`SELECT count(*) FROM mycenae.ts_keyspace WHERE token(key) >= 0;`).Scan(&count2); err != nil {
		log.Println(err)
	}
	return count1 + count2
}

func (ts *cassTs) KeyspaceTables(keyspace string) (tables []string) {
	iter := ts.cql.Query(
		cqlKeyspaceTables,
		keyspace,
	).Iter()

	var table string

	for iter.Scan(&table) {
		tables = append(tables, table)
	}
	if err := iter.Close(); err != nil {
		log.Println(err)
	}
	return
}

func (ts *cassTs) Exists(keyspace string) bool {
	it := ts.cql.Query(cqlExists, keyspace).Iter()
	var err error
	var count int
	var scanned string
	for it.Scan(&scanned) {
		if keyspace == scanned {
			count++;
		}
	}
	if err = it.Close(); err != nil {
		log.Println(err)
	}
	return err == nil && count == 1
}

//func (ts *cassTs) Exists(keyspace string) bool {
//	var count int
//	err := ts.cql.Query(
//		cqlExists,
//		keyspace,
//	).Scan(&count)
//	return err == nil && count == 1
//}

func (ts *cassTs) ExistsInformation(keyspace string, name string, replication_factor int, datacenter string, ttl int, tuuid bool, contact string) bool {
	var err error
	var count int
	var scName, scDatacenter, scContact string
	var scRepFactor, scTTL int
	var scTUUID bool
	it := ts.cql.Query(
		cqlExistsInformation,
		keyspace,
	).Iter()
	for it.Scan(&scName, &scDatacenter, &scContact, &scRepFactor, &scTTL, &scTUUID) {
		if scName == name && scDatacenter == datacenter && scContact == contact &&
		   scRepFactor == replication_factor && scTTL == ttl &&
		   scTUUID == tuuid {
			count++;
		}
	}
	if err = it.Close(); err != nil {
		log.Println(err)
	}
	return err == nil && count == 1

}

//func (ts *cassTs) ExistsInformation(keyspace string, name string, replication_factor int, datacenter string, ttl int, tuuid bool, contact string) bool {
//	var count int
//	err := ts.cql.Query(
//		cqlExistsInformation,
//		keyspace,
//		name,
//		replication_factor,
//		datacenter,
//		ttl,
//		tuuid,
//		contact,
//	).Scan(&count)
//	return err == nil && count == 1
//
//}

func (ts *cassTs) TableProperties(keyspace string, table string) TableProperties {
	var caching, compaction, compression map[string]string
	var speculative_retry, comment string
	var default_time_to_live, gc_grace_seconds, max_index_interval, memtable_flush_period_in_ms,
	min_index_interval int
	var bloom_filter_fp_chance, dclocal_read_repair_chance, read_repair_chance float64

	if err := ts.cql.Query(cqlTableProperties,
		keyspace,
		table,
	).Scan(&bloom_filter_fp_chance, &caching, &comment, &compaction, &compression, &dclocal_read_repair_chance,
		&default_time_to_live, &gc_grace_seconds, &max_index_interval, &memtable_flush_period_in_ms, &min_index_interval,
		&read_repair_chance, &speculative_retry); err != nil {
		log.Println(err)
	}

	return TableProperties{
		Bloom_filter_fp_chance:      bloom_filter_fp_chance,
		Caching:                     caching,
		Comment:                     comment,
		Compaction:                  compaction,
		Compression:                 compression,
		Dclocal_read_repair_chance:  dclocal_read_repair_chance,
		Default_time_to_live:        default_time_to_live,
		Gc_grace_seconds:            gc_grace_seconds,
		Max_index_interval:          max_index_interval,
		Memtable_flush_period_in_ms: memtable_flush_period_in_ms,
		Min_index_interval:          min_index_interval,
		Read_repair_chance:          read_repair_chance,
		Speculative_retry:           speculative_retry,
	}
}

func (ts *cassTs) Drop(keyspace string) bool {

	err := ts.cql.Query(
		fmt.Sprintf(cqlDropKS, keyspace),
	).Exec()

	return err == nil

}

func (ts *cassTs) Delete(keyspace string) bool {

	err := ts.cql.Query(
		fmt.Sprintf(cqlDeleteKS, keyspace),
	).Exec()

	return err == nil

}

func (ts *cassTs) Insert(keyspace string) bool {

	err := ts.cql.Query(
		fmt.Sprintf(cqlInsertKS, keyspace, keyspace),
	).Exec()

	return err == nil

}

func (ts *cassTs) KsAttributes(keyspace string) KeyspaceAttributes {
	var name, datacenter, contact, replication_factor_meta string
	var replication_factor, ks_ttl int
	var ks_tuuid bool

	if err := ts.cql.Query(cqlSelectKS,
		keyspace,
	).Scan(&name, &replication_factor, &datacenter, &ks_ttl, &ks_tuuid, &contact, &replication_factor_meta); err != nil {
		log.Println(err)
	}
	return KeyspaceAttributes{
		Name:                    name,
		Datacenter:              datacenter,
		Contact:                 contact,
		Replication_factor:      replication_factor,
		Ks_ttl:                  ks_ttl,
		Ks_tuuid:                ks_tuuid,
		Replication_factor_meta: replication_factor_meta,
	}
}

func (ts *cassTs) KeyspaceProperties(keyspace string) KeyspaceProperties {
	var keyspace_name string
	var durable_writes bool
	var replication map[string]string

	if err := ts.cql.Query(cqlKeyspaceProperties,
		keyspace,
	).Scan(&keyspace_name, &durable_writes, &replication); err != nil {
		log.Println(err)
	}
	return KeyspaceProperties{
		Keyspace_name:  keyspace_name,
		Durable_writes: durable_writes,
		Replication: replication,
	}
}
