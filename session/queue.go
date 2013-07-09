package session

type QueueItem struct {
  data []byte
  priority int
  index int
}

type Queue []*QueueItem

func (self Queue) Len() int { return len(self) }

func (self Queue) Less(i, j int) bool {
  return self[i].priority > self[j].priority
}

func (self Queue) Swap(i, j int) {
  self[i], self[j] = self[j], self[i]
  self[i].index = i
  self[j].index = j
}

func (self *Queue) Push(e interface{}) {
  n := len(*self)
  item := e.(*QueueItem)
  item.index = n
  *self = append(*self, item)
}

func (self *Queue) Pop() interface{} {
  old := *self
  n := len(old)
  item := old[n - 1]
  item.index = -1
  *self = old[0 : n - 1]
  return item
}
