// Copyright 2021 iLogtail Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ilogtail

import (
	"github.com/alibaba/ilogtail/pkg/protocol"
)

// Aggregator是一个实现聚合器插件的接口. RunningAggregator包装这个接口并保证Push,
// Add, Reset接口不会被并发调用。因此当实现Aggregator的时候并不需要加锁
// 类似logstore，skywalking，opentelemetry会实现接口
type Aggregator interface {
	// Init为一些系统资源调用，例如socket，mutex等，返回flush间隔和错误标记，如果间隔是0，使用默认间隔
	Init(Context, LogGroupQueue) (int, error)

	// Description基于输入返回一个描述信息
	Description() string

	// Add metrics到聚合器中
	Add(log *protocol.Log) error

	// Flush将目前的聚合信息推到accumulator里面
	Flush() []*protocol.LogGroup

	// Reset重置聚合缓存和聚合信息
	Reset()
}
