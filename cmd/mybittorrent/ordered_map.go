package main

type OrderedMap struct {
    keys   []string
    values map[string]any
}

func NewOrderedMap() *OrderedMap {
    return &OrderedMap{
        keys:   make([]string, 0),
        values: make(map[string]any),
    }
}

func (om *OrderedMap) Set(key string, value any) {
    if _, exists := om.values[key]; !exists {
        om.keys = append(om.keys, key)
    }
    om.values[key] = value
}

func (om *OrderedMap) Get(key string) (any, bool) {
    value, exists := om.values[key]
    return value, exists
}

func (om *OrderedMap) Keys() []string {
    return om.keys
}

func (om *OrderedMap) GetMap() map[string]any {
  plainMap := make(map[string]any)
  for k, v := range om.values {
    if orig, ok := v.(*OrderedMap); ok {
      plainMap[k] = orig.GetMap()
    } else {
      plainMap[k] = v
    }
  }
  return plainMap
}
