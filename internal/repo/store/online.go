package store

import "sync"

type OKv struct {
	Id   string
	Name string
}

var (
	onlineMu    sync.RWMutex
	onlineStore = make(map[string][]OKv)
)

func CacheOnline(uid string, messages []OKv) {
	onlineMu.Lock()
	defer onlineMu.Unlock()
	onlineStore[uid] = messages
}

func DeleteOnline(uid string) {
	onlineMu.Lock()
	defer onlineMu.Unlock()
	delete(onlineStore, uid)
}

func GetOnline(uid string) []OKv {
	if result, ok := onlineStore[uid]; ok {
		return result
	}
	return make([]OKv, 0)
}
