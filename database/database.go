package database

import "github.com/codenotary/immudb/embedded/store"

const Name = ".db"

func Get(key []byte) ([]byte, error) {
	st, err := store.Open(Name, store.DefaultOptions())
	if err != nil {
		return nil, err
	}
	defer st.Close()

	tx, err := st.NewTx()
	if err != nil {
		return nil, err
	}
	defer tx.Cancel()

	valRef, err := tx.Get(key)
	if err != nil {
		return nil, err
	}

	val, err := valRef.Resolve()
	if err != nil {
		return nil, err
	}

	return val, nil
}

func Set(key []byte, val []byte) error {
	st, err := store.Open(Name, store.DefaultOptions())
	if err != nil {
		return err
	}
	defer st.Close()

	tx, err := st.NewTx()
	if err != nil {
		return err
	}
	defer tx.Cancel()

	err = tx.Set(key, nil, val)
	if err != nil {
		return err
	}

	_, err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
