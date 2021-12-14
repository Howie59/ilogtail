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

package defaultone

import (
	"sync"

	"github.com/alibaba/ilogtail"
	"github.com/alibaba/ilogtail/pkg/protocol"
	"github.com/alibaba/ilogtail/pkg/util"
)

const (
	MaxLogCount     = 1024
	MaxLogGroupSize = 3 * 1024 * 1024
)

// AggregatorDefault是插件系统里面的默认aggregator，它有两个用途：
// 	1.如果插件配置中没有特定的aggregator，它默认会被添加
// 	2.其他aggregator可以把它当做基础的aggregator
// For inner usage, note about following information.
// There is a quick flush design in AggregatorDefault, which is implemented
// in Add method (search p.queue.Add in current file). Therefore, not all
// LogGroups are returned through Flush method.
// If you want to do some operations (such as adding tags) on LogGroups returned
// by AggregatorDefault in your own aggregator, you should do some extra works,
// just see the sample code in doc.go.
type AggregatorDefault struct {
	MaxLogGroupCount int    // 触发flush操作的最大日志组量
	MaxLogCount      int    // 日志组中的最大量
	PackFlag         bool   // 是否添加配置名为一个tag
	Topic            string // 输出的topic

	pack            string
	defaultLogGroup []*protocol.LogGroup
	packID          int64
	logstore        string
	Lock            *sync.Mutex
	context         ilogtail.Context
	queue           ilogtail.LogGroupQueue
	nowLoggroupSize int
}

// Init方法在工作前会被触发
//  1. context存储Logstore配置的元信息
//  2. 当缓存中达到最大量的时候，que是flush LogGroup的一个传输通道
func (p *AggregatorDefault) Init(context ilogtail.Context, que ilogtail.LogGroupQueue) (int, error) {
	p.context = context
	p.queue = que
	if p.PackFlag {
		p.pack = util.NewPackIDPrefix(context.GetConfigName())
	}
	return 0, nil
}

func (*AggregatorDefault) Description() string {
	return "default aggregator for logtail"
}

// evaluateLogSize用于评估日志的大小
func (*AggregatorDefault) evaluateLogSize(log *protocol.Log) int {
	var logSize = 6
	for _, logC := range log.Contents {
		logSize += 5 + len(logC.Key) + len(logC.Value)
	}
	return logSize
}

// Add adds @log to aggregator.
// It uses defaultLogGroup to store log groups which contain logs as following:
// defaultLogGroup => [LG1: log1->log2->log3] -> [LG2: log1->log2->log3] -> ..
// The last log group is set as nowLogGroup, @log will be appended to nowLogGroup
// if the size and log count of the log group don't exceed limits (MaxLogCount and
// MAX_LOG_GROUP_SIZE).
// When nowLogGroup exceeds limits, Add creates a new log group and switch nowLogGroup
// to it, then append @log to it.
// When the count of log group reaches MaxLogGroupCount, the first log group will
// be popped from defaultLogGroup list and add to queue (after adding pack_id tag).
// Add returns any error encountered, nil means success.
//
// @return error. **For inner usage, must handle this error!!!!**
func (p *AggregatorDefault) Add(log *protocol.Log) error {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	if len(p.defaultLogGroup) == 0 {
		p.nowLoggroupSize = 0
		p.defaultLogGroup = append(p.defaultLogGroup, &protocol.LogGroup{})
	}
	nowLogGroup := p.defaultLogGroup[len(p.defaultLogGroup)-1]
	logSize := p.evaluateLogSize(log)

	// When current log group is full (log count or no more capacity for current log),
	// allocate a new log group.
	if len(nowLogGroup.Logs) >= p.MaxLogCount || p.nowLoggroupSize+logSize > MaxLogGroupSize {
		// The number of log group exceeds limit, make a quick flush.
		if len(p.defaultLogGroup) == p.MaxLogGroupCount {
			// try to send
			p.addPackID(p.defaultLogGroup[0])
			p.defaultLogGroup[0].Category = p.logstore
			if len(p.Topic) > 0 {
				p.defaultLogGroup[0].Topic = p.Topic
			}

			// 为了避免大日志来了之后变为瓶颈，日志会迅速清理
			if err := p.queue.Add(p.defaultLogGroup[0]); err == nil {
				// add success, remove head log group
				p.defaultLogGroup = p.defaultLogGroup[1:]
			} else {
				return err
			}
		}
		// New log group, reset size.
		p.nowLoggroupSize = 0
		p.defaultLogGroup = append(p.defaultLogGroup, &protocol.LogGroup{})
		nowLogGroup = p.defaultLogGroup[len(p.defaultLogGroup)-1]
	}

	// add log size
	p.nowLoggroupSize += logSize
	nowLogGroup.Logs = append(nowLogGroup.Logs, log)
	return nil
}

// addPackID adds __pack_id__ into logGroup.LogTags if it is not existing at last.
func (p *AggregatorDefault) addPackID(logGroup *protocol.LogGroup) {
	if !p.PackFlag {
		return
	}
	if len(logGroup.LogTags) == 0 || logGroup.LogTags[len(logGroup.LogTags)-1].GetKey() != util.PackIDTagKey {
		logGroup.LogTags = append(logGroup.LogTags, util.NewLogTagForPackID(p.pack, &p.packID))
	}
}

// Flush ...
func (p *AggregatorDefault) Flush() []*protocol.LogGroup {
	p.Lock.Lock()
	if len(p.defaultLogGroup) == 0 {
		p.Lock.Unlock()
		return nil
	}
	p.nowLoggroupSize = 0
	logGroupList := p.defaultLogGroup
	p.defaultLogGroup = make([]*protocol.LogGroup, 0, p.MaxLogGroupCount)
	p.Lock.Unlock()

	for i, logGroup := range logGroupList {
		// check if last is empty
		if len(logGroup.Logs) == 0 {
			logGroupList = logGroupList[0:i]
			break
		}
		p.addPackID(logGroup)
		logGroup.Category = p.logstore
		if len(p.Topic) > 0 {
			logGroup.Topic = p.Topic
		}
	}
	return logGroupList
}

// Reset ...
func (p *AggregatorDefault) Reset() {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	p.defaultLogGroup = make([]*protocol.LogGroup, 0)
}

// InitInner initializes instance for other aggregators.
func (p *AggregatorDefault) InitInner(packFlag bool, packString string, lock *sync.Mutex, logstore string, topic string, maxLogCount int, maxLoggroupCount int) {
	p.PackFlag = packFlag
	p.MaxLogCount = maxLogCount
	p.MaxLogGroupCount = maxLoggroupCount
	p.Lock = lock
	p.Topic = topic
	p.logstore = logstore
	if p.PackFlag {
		p.pack = util.NewPackIDPrefix(packString)
	}
}

// NewAggregatorDefault create a default aggregator with default value.
func NewAggregatorDefault() *AggregatorDefault {
	return &AggregatorDefault{
		defaultLogGroup:  make([]*protocol.LogGroup, 0),
		MaxLogGroupCount: 4,
		MaxLogCount:      MaxLogCount,
		PackFlag:         true,
		Lock:             &sync.Mutex{},
	}
}

// Register the plugin to the Aggregators array.
func init() {
	ilogtail.Aggregators["aggregator_default"] = func() ilogtail.Aggregator {
		return NewAggregatorDefault()
	}
}
