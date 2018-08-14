/*
 *  Copyright (c) 2018 Uber Technologies, Inc.
 *
 *     Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package terraform

// InitLocal downloads the remote state files to the local session so
// that changes can be made to the state files locally without being
// pushed.
func (s *Session) InitLocal() (Result, error) {
	if !s.Initialized() {
		if result, err := s.Init(); err != nil {
			return result, err
		}
	}

	args := []string{"remote", "config", "-disable"}

	process, err := s.terraformCommand(args, []int{0})
	if err != nil {
		return nil, err
	}

	if err := process.Run(); err != nil {
		return nil, err
	}

	return &terraformResult{
		process: process,
	}, nil
}
