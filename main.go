package main

import (
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
	prev      *entry
	next      *entry
}

type LRUCache struct {
	capacity   int
	cache      map[int]*entry
	head, tail *entry
	mutex      sync.Mutex
	expiration time.Duration
}

func Constructor(capacity int, expiration time.Duration) LRUCache {
	cache := LRUCache{
		capacity:   capacity,
		cache:      make(map[int]*entry),
		expiration: expiration,
	}
	go cache.startEvictionRoutine()
	return cache
}

func (this *LRUCache) Get(key int) int {
	this.mutex.Lock()
	defer this.mutex.Unlock()

	if elem, ok := this.cache[key]; ok {
		entry := elem
		entry.timestamp = time.Now()
		if time.Since(entry.timestamp) > this.expiration {
			this.evict(key)
			return -1
		}
		this.moveToFront(entry)
		return entry.value
	}
	return -1
}

func (this *LRUCache) Set(key int, value int) {
	this.mutex.Lock()
	defer this.mutex.Unlock()

	if elem, ok := this.cache[key]; ok {
		entry := elem
		entry.value = value
		entry.timestamp = time.Now()
		this.moveToFront(entry)
	} else {
		if len(this.cache) >= this.capacity {
			this.evict(this.tail.key)
		}
		newEntry := &entry{key: key, value: value, timestamp: time.Now()}
		this.cache[key] = newEntry
		this.addToFront(newEntry)
	}
}

func (this *LRUCache) evict(key int) {
	if elem, ok := this.cache[key]; ok {
		delete(this.cache, key)
		this.remove(elem)
		log.Printf("Evicted key: %d\n", key)
	}
}

func (this *LRUCache) moveToFront(entry *entry) {
	this.remove(entry)
	this.addToFront(entry)
}

func (this *LRUCache) addToFront(entry *entry) {
	entry.prev = nil
	entry.next = this.head
	if this.head != nil {
		this.head.prev = entry
	}
	this.head = entry
	if this.tail == nil {
		this.tail = entry
	}
}

func (this *LRUCache) remove(entry *entry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		this.head = entry.next
	}
	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		this.tail = entry.prev
	}
}

func (this *LRUCache) startEvictionRoutine() {
	ticker := time.Tick(1 * time.Second)
	for range ticker {
		this.mutex.Lock()
		for key, elem := range this.cache {
			if time.Since(elem.timestamp) > this.expiration {
				this.evict(key)
			}
		}
		this.mutex.Unlock()
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
