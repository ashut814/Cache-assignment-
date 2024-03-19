package main

import (
	"container/list"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type entry struct {
	key       int
	value     int
	timestamp time.Time 
}

type LRUCache struct {
	capacity  int
	cache     map[int]*list.Element
	eviction  *list.List
	mutex     sync.Mutex
	expiration time.Duration 
}

func Constructor(capacity int, expiration time.Duration) LRUCache {
	return LRUCache{
		capacity:  capacity,
		cache:     make(map[int]*list.Element),
		eviction:  list.New(),
		expiration: expiration, 
	}
}

func (this *LRUCache) Get(key int) int {
	this.mutex.Lock()
	defer this.mutex.Unlock()

	if elem, ok := this.cache[key]; ok {
		entry := elem.Value.(*entry)
		if time.Since(entry.timestamp) > this.expiration {
			this.evict(key) // Evict the key if expired
			return -1
		}
		this.eviction.MoveToFront(elem)
		return entry.value
	}
	return -1
}

func (this *LRUCache) Set(key int, value int) {
	this.mutex.Lock()
	defer this.mutex.Unlock()

	if elem, ok := this.cache[key]; ok {
		entry := elem.Value.(*entry)
		entry.value = value
		entry.timestamp = time.Now()
		this.eviction.MoveToFront(elem)
	} else {
		if len(this.cache) >= this.capacity {
			back := this.eviction.Back()
			if back != nil {
				this.evict(back.Value.(*entry).key)
			}
		}
		newEntry := &entry{key: key, value: value, timestamp: time.Now()}
		elem := this.eviction.PushFront(newEntry)
		this.cache[key] = elem
	}
}

func (this *LRUCache) evict(key int) {
	if elem, ok := this.cache[key]; ok {
		delete(this.cache, key)
		this.eviction.Remove(elem)
	}
}

type CacheHandler struct {
	cache *LRUCache
	mutex sync.Mutex
}

func (h *CacheHandler) SetHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var data struct {
		Key   int `json:"key"`
		Value int `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.cache.Set(data.Key, data.Value)

	w.WriteHeader(http.StatusOK)
}

func (h *CacheHandler) GetHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	keyStr := r.URL.Query().Get("key")
	key, err := strconv.Atoi(keyStr)
	if err != nil {
		http.Error(w, "Invalid key", http.StatusBadRequest)
		return
	}

	value := h.cache.Get(key)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"value":      value,
		"expiration": time.Now().Add(h.cache.expiration).Unix(),
	})
}

func main() {
	cache := Constructor(1024, 5*time.Second)

	cacheHandler := &CacheHandler{cache: &cache}

	http.HandleFunc("/cache/set", cacheHandler.SetHandler)
	http.HandleFunc("/cache/get", cacheHandler.GetHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
