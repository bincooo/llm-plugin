package store

import "sync"

type OKeyv struct {
	Id   string
	Name string
}

var (
	omu         sync.RWMutex
	onlineStore = make(map[string][]OKeyv)
)

func CacheOnline(uid string, messages []OKeyv) {
	omu.Lock()
	defer omu.Unlock()
	onlineStore[uid] = messages
}

func DeleteOnline(uid string) {
	omu.Lock()
	defer omu.Unlock()
	delete(onlineStore, uid)
}

func GetOnline(uid string) []OKeyv {
	if result, ok := onlineStore[uid]; ok {
		return result
	}
	return make([]OKeyv, 0)
}
