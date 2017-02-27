package docker

import (
	"fmt"
	"sync/atomic"
)

func (m *Monitor) staticPortSlotAllocated(i int) bool {
	return atomic.LoadInt32(&m.availableStaticPorts[i]) == 0
}

func (m *Monitor) deallocateStaticPort(i int) {
	atomic.StoreInt32(&m.availableStaticPorts[i], 0)
}

func (m *Monitor) deallocateStaticPortByContainer(c *MemContainer) {
	if c.StaticPortMin != 0 {
		index := c.StaticPortMin / m.staticPortPerContainer
		m.deallocateStaticPort(index)
		c.StaticPortMin = 0
		c.StaticPortMax = 0
	}
}

func (m *Monitor) nStaticPortSlot() int {
	return (m.staticPortMax - m.staticPortMin) / m.staticPortPerContainer
}

func (m *Monitor) resetAllStaticPortSlot() {
	maxSlot := m.nStaticPortSlot()
	for i := 0; i < maxSlot; i++ {
		atomic.StoreInt32(&m.availableStaticPorts[i], 0)
	}
}

func (m *Monitor) allocateStaticPortSlot() (PortRange, error) {
	maxSlot := m.nStaticPortSlot()
	for i := 0; i < maxSlot; i++ {
		if atomic.CompareAndSwapInt32(&m.availableStaticPorts[i], 0, 1) {
			min := m.staticPortMin + i*m.staticPortPerContainer
			max := m.staticPortMin + (i+1)*m.staticPortPerContainer - 1
			return PortRange{min: min, max: max}, nil
		}
	}
	return PortRange{0, 0}, fmt.Errorf("can not find available slot")
}
