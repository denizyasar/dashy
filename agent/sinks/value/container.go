package value

import "time"

type Container map[string]Value

func New() Container {
	return make(Container)
}

func (c Container) Map(mapping map[string]string) Container {
	newContainer := make(Container)
	for k, v := range c {
		if newKey, exists := mapping[k]; exists {
			newContainer[newKey] = v
		} else {
			newContainer[k] = v
		}
	}
	return newContainer
}

func (c Container) Add(name string, t time.Time, v string) Container {
	c[name] = Value{
		Timestamp: t,
		Value:     v,
	}
	return c
}

type Value struct {
	Timestamp time.Time `json:"timestamp"`
	Value     string    `json:"value"`
}
