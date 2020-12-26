package rosedb

import (
	"rosedb/index"
	"rosedb/storage"
)

//---------字符串相关操作接口-----------

//将字符串值 value 关联到 key
//如果 key 已经持有其他值，SET 就覆写旧值
func (db *RoseDB) Set(key, value []byte) error {

	if err := db.checkKeyValue(key, value); err != nil {
		return err
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	e := storage.NewEntryNoExtra(key, value, String, StringSet)
	idx, err := db.store(e)
	if err != nil {
		return err
	}

	if err := db.buildIndex(e, idx); err != nil {
		return err
	}

	return nil
}

//SetNx 是SET if Not Exists(如果不存在，则 SET)的简写
//只在键 key 不存在的情况下， 将键 key 的值设置为 value
//若键 key 已经存在， 则 SetNx 命令不做任何动作
func (db *RoseDB) SetNx(key, value []byte) error {

	if oldVal, err := db.Get(key); oldVal != nil || err != nil {
		return err
	}

	return db.Set(key, value)
}

func (db *RoseDB) Get(key []byte) ([]byte, error) {
	keySize := uint32(len(key))
	if keySize == 0 {
		return nil, ErrEmptyKey
	}

	node := db.idxList.Get(key)
	if node == nil {
		return nil, ErrKeyNotExist
	}

	idx := node.Value().(*index.Indexer)
	if idx == nil {
		return nil, ErrNilIndexer
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	//如果key和value均在内存中，则取内存中的value
	if db.config.IdxMode == KeyValueRamMode {
		return idx.Meta.Value, nil
	}

	//如果只有key在内存中，那么需要从db file中获取value
	if db.config.IdxMode == KeyOnlyRamMode {
		df := db.activeFile
		if idx.FileId != db.activeFileId {
			df = db.archFiles[idx.FileId]
		}

		if e, err := df.Read(idx.Offset); err != nil {
			return nil, err
		} else {
			return e.Meta.Value, nil
		}
	}

	return nil, ErrKeyNotExist
}

//将键 key 的值设为 value ， 并返回键 key 在被设置之前的旧值。
func (db *RoseDB) GetSet(key, val []byte) (res []byte, err error) {

	if res, err = db.Get(key); err != nil {
		return
	}

	if err = db.Set(key, val); err != nil {
		return
	}

	return
}

//如果key存在，则将value追加至原来的value末尾
//如果key不存在，则相当于Set方法
func (db *RoseDB) Append(key, value []byte) error {

	if err := db.checkKeyValue(key, value); err != nil {
		return err
	}

	e, err := db.Get(key)
	if err != nil {
		return err
	}

	if e != nil {
		e = append(e, value...)
	} else {
		e = value
	}

	return db.Set(key, e)
}

//返回key存储的字符串值的长度
func (db *RoseDB) StrLen(key []byte) int {
	e := db.idxList.Get(key)
	if e != nil {
		idx := e.Value().(*index.Indexer)
		return int(idx.Meta.ValueSize)
	}

	return 0
}