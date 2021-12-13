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

// Flusher ...
// Sample flusher implementation: see plugin_manager/flusher_sls.gox.
type Flusher interface {
	// Init called for init some system resources, like socket, mutex...
	Init(Context) error

	// Description returns a one-sentence description on the Input.
	Description() string

	// IsReady checks if flusher is ready to accept more data.
	// @projectName, @logstoreName, @logstoreKey: meta of the corresponding data.
	// Note: If SetUrgent is called, please make some adjustment so that IsReady
	//   can return true to accept more data in time and config instance can be
	//   stopped gracefully.
	IsReady(projectName string, logstoreName string, logstoreKey int64) bool

	// Flush flushes data to destination, such as SLS, console, file, etc.
	// It is expected to return no error at most time because IsReady will be called
	// before it to make sure there is space for next data.
	Flush(projectName string, logstoreName string, configName string, logGroupList []*protocol.LogGroup) error

	// SetUrgent indicates the flusher that it will be destroyed soon.
	// @flag indicates if main program (Logtail mostly) will exit after calling this.
	//
	// Note: there might be more data to flush after SetUrgent is called, and if flag
	//   is true, these data will be passed to flusher through IsReady/Flush before
	//   program exits.
	//
	// Recommendation: set some state flags in this method to guide the behavior
	//   of other methods.
	SetUrgent(flag bool)

	// Stop停掉flusher和release资源。表示可以做清理工作了，包括：
	//   1. Flush缓存但是还没清理的数据。因为flusher包含了一些聚合或者缓存，清理掉cache部分非常重要，否则数据将会丢失
	//   2. Release开启的资源：goroutines，file handles，connections等等
	// flusher仅仅可以做GC清理之后的事情
	Stop() error
}
