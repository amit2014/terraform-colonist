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

package colonist

import (
	"fmt"
	"path/filepath"

	"github.com/uber/terraform-colonist/colonist/conf"
	"github.com/uber/terraform-colonist/colonist/logger"
	"github.com/uber/terraform-colonist/colonist/tvm"
	"github.com/uber/terraform-colonist/colonist/utils"
)

// Colony is a collection of Terraform modules, based on configuration.
//
// Modules may be invoked with various parameters, which are either
// provided by the user at runtime, or predefined in configuration.
//
// The combination of a module, along with a map of variable values, is
// called an "execution".
//
// Executions can have dependencies between each other (again, defined
// in the configuration). Based on dependencies, all modules can be
// planned or applied concurrently.
//
type Colony struct {
	config            *conf.Colony
	sessions          *SessionRepo
	terraformVersions *tvm.VersionRepo
}

// NewColony returns a new instance of Colony.
func NewColony(config conf.Colony) (*Colony, error) {
	logger.Trace.Println("colony: initializing")

	colony := &Colony{}

	versionRepo, err := tvm.NewVersionRepoForCurrentSystem("")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tvm: %v", err)
	}

	sessionRepoPath := filepath.Join(config.SessionRepoDir, ".tfcolony")
	sessions, err := NewSessionRepo(colony, sessionRepoPath, utils.ULIDString)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize session repository: %v", err)
	}

	colony.config = &config
	colony.sessions = sessions
	colony.terraformVersions = versionRepo

	// validate config
	if errs := colony.config.Validate(); errs != nil {
		return nil, errs
	}

	// check dependency graph is all good
	if _, err := colony.executions(nil, NoUserVariables()).graph(); err != nil {
		return nil, err
	}

	if config.Hooks.Startup == nil {
		return colony, nil
	}

	session, err := colony.sessions.Current()
	if err != nil {
		return nil, err
	}
	for _, hook := range config.Hooks.Startup {
		if err := runCommandkAndSetEnvironment(session.path, hook); err != nil {
			return nil, fmt.Errorf("error running Startup hook: %v", err)
		}
	}

	return colony, nil
}

// executions returns a set of executions for modules registered in this
// colony.
func (c *Colony) executions(moduleNames []string, userVars *UserVariables) executionSet {
	results := executionSet{}
	for _, m := range c.modules(moduleNames) {
		results = append(results, m.executions(userVars)...)
	}
	return results
}

// modules creates a list of modules based on the config.
func (c *Colony) modules(moduleNames []string) []*module {
	results := []*module{}
	for _, moduleConfig := range c.config.Modules {
		// skip, if we're filtering and this module doesn't match the filter
		if moduleNames != nil && !utils.StringSliceContains(moduleNames, moduleConfig.Name) {
			logger.Trace.Printf("colony: ignoring module %v as it does not match filter", moduleConfig.Name)
			continue
		}
		results = append(results, newModule(moduleConfig))
	}
	return results
}

// Plan does a Terraform plan for every possible execution, in
// parallel, ignoring dependencies.
func (c *Colony) Plan(moduleNames []string, userVars *UserVariables, detach bool) (<-chan string, <-chan *Result, error) {
	logger.Trace.Println("colony: running Plan")

	// Binds user vars
	boundExecutions, err := c.executions(moduleNames, userVars).bindAll(userVars.Values)
	if err != nil {
		return nil, nil, err
	}

	// Get session
	session, err := c.sessions.Current()
	if err != nil {
		return nil, nil, err
	}

	return session.plan(boundExecutions, detach)
}

// Apply does a Terraform apply for every possible execution,
// in parallel, taking into consideration dependencies. It returns an
// error if it is unable to start, e.g. due to a missing required
// variable.
func (c *Colony) Apply(moduleNames []string, userVars *UserVariables) (<-chan string, <-chan *Result, error) {
	logger.Trace.Println("colony: running Apply")

	// Bind user vars
	boundExecutions, err := c.executions(moduleNames, userVars).bindAll(userVars.Values)
	if err != nil {
		return nil, nil, err
	}

	// Get session
	session, err := c.sessions.Current()
	if err != nil {
		return nil, nil, err
	}

	var applyFn func([]*boundExecution) (<-chan string, <-chan *Result, error)
	if moduleNames != nil {
		applyFn = session.apply
	} else {
		applyFn = session.applyWithGraph
	}

	return applyFn(boundExecutions)
}
