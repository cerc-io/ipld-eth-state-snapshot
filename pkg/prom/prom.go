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
}

func RegisterGaugeFunc(name string, function func() float64) {
	promauto.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: statsSubsystem,
			Name:      name,
			Help:      name,
		}, function)
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

// AddStorageNodeCount increments the number of storage nodes processed
func AddStorageNodeCount(count int) {
	if metrics && count > 0 {
		storageNodeCount.Add(float64(count))
	}
}

func Enabled() bool {
	return metrics
}
