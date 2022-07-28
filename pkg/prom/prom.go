// VulcanizeDB
// Copyright © 2022 Vulcanize

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package prom

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "ipld_eth_state_snapshot"

	connSubsystem  = "connections"
	statsSubsystem = "stats"
)

var (
	metrics bool

	stateNodeCount   prometheus.Counter
	storageNodeCount prometheus.Counter
	codeNodeCount    prometheus.Counter

	activeIteratorCount prometheus.Gauge
)

func Init() {
	metrics = true

	stateNodeCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: statsSubsystem,
		Name:      "state_node_count",
		Help:      "Number of state nodes processed",
	})

	storageNodeCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: statsSubsystem,
		Name:      "storage_node_count",
		Help:      "Number of storage nodes processed",
	})

	codeNodeCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: statsSubsystem,
		Name:      "code_node_count",
		Help:      "Number of code nodes processed",
	})

	activeIteratorCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: statsSubsystem,
		Name:      "active_iterator_count",
		Help:      "Number of active iterators",
	})
}

// RegisterDBCollector create metric collector for given connection
func RegisterDBCollector(name string, db DBStatsGetter) {
	if metrics {
		prometheus.Register(NewDBStatsCollector(name, db))
	}
}

// IncStateNodeCount increments the number of state nodes processed
func IncStateNodeCount() {
	if metrics {
		stateNodeCount.Inc()
	}
}

// IncStorageNodeCount increments the number of storage nodes processed
func IncStorageNodeCount() {
	if metrics {
		storageNodeCount.Inc()
	}
}

// IncCodeNodeCount increments the number of code nodes processed
func IncCodeNodeCount() {
	if metrics {
		codeNodeCount.Inc()
	}
}

// IncActiveIterCount increments the number of active iterators
func IncActiveIterCount() {
	if metrics {
		activeIteratorCount.Inc()
	}
}

// DecActiveIterCount decrements the number of active iterators
func DecActiveIterCount() {
	if metrics {
		activeIteratorCount.Dec()
	}
}
