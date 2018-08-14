# Terraform Colonist

Colonist is a tool for managing multiple Terraform executions as a single command.

Features:

* Declarative configuration for modules to execute
* Dependencies between modules
* Fast, concurrent executions of Terraform operations
* Safe Terraform upgrades and state file migrations

NOTE: Colonist is currently experimental.

## Getting started

**Installation**

Install Colonist using go get:

```
go get -u github.com/uber/terraform-colonist/colonist/cli/colony
```

This will install a binary called `colony` in your `$GOPATH/bin`.

**Configuration**

Colonist looks for a configuration file called `colony.yaml` in the current or parent directories. It is recommended to place this file in the same top-level directory of your project where the Terraform code exists (e.g. `terraform/colony.yaml`).

An example colony configuration could look like:

```
---

terraform:
  version: 0.11.7

hooks:
  startup:
    - command: assume-role --role terraform
      set_env: true

modules:
  - name: app
    path: core/app
    deps:
      - module: users
      - module: vpc
    remote:
      backend_config:
        bucket: acme-terraform-states
        key: "{{.aws_region}}/app-{{.environment}}.tfstate"
        region: us-east-1
    variables:
      - name: region
      - name: environment
        values: [dev, prod]

  - name: database
    path: core/database
    remote:
      backend_config:
        bucket: acme-terraform-states
        key: "{{.aws_region}}/database-{{.environment}}.tfstate"
        region: us-east-1
    variables:
      - name: region
      - name: environment
        values: [dev, prod]

  - name: mgmt
    path: core/mgmt
    deps:
      - module: vpc
        variables:
          environment: mgmt  # depends on vpc/mgmt
    remote:
      backend_config:
        bucket: acme-terraform-states
        key: "{{.aws_region}}/mgmt-{{.environment}}.tfstate"
        region: us-east-1
    variables:
      - name: region

  - name: users
    path: core/users
    remote:
      backend_config:
        bucket: acme-terraform-states
        key: global/users
        region: us-east-1

  - name: vpc
    path: core/vpc
    remote:
      backend_config:
        bucket: acme-terraform-states
        key: "{{.aws_region}}/vpc-{{.environment}}.tfstate"
        region: us-east-1
    variables:
      - name: region
      - name: environment
        values: [mgmt, dev, prod]
```

**Planning**

You can run a plan across all modules by doing:

```
colony plan --region us-east-1
```

`--region` in this example is one of the variables defined in the module configuration above with no predefined value, so it must be provided at the command line.

Colonist will show the results of the plan for each execution:


```
> colony plan --region us-east-1
users: OK No changes (7s)
vpc-mgmt-us-east-1: OK No changes (15s)
vpc-dev-us-east-1: OK No changes (31s)
vpc-prod-us-east-1: OK No changes (28s)
database-dev-us-east-1: OK No changes (9s)
database-prod-us-east-1: OK No changes (10s)
app-dev-us-east-1: OK No changes (10s)
app-prod-us-east-1: OK No changes (11s)
mgmt-us-east-1: OK No changes (43s)
> 
```

If there is a change, the plan will be shown, e.g.:


```
> colony plan --region us-east-1 --modules app
app-dev-us-east-1: OK Changes (10s)

  ~ module.app.aws_s3_bucket.app-data
      versioning.0.enabled: "false" => "true"

app-prod-us-east-1: OK Changes (11s)

  ~ module.app.aws_s3_bucket.app-data
      versioning.0.enabled: "false" => "true"
> 
```

**Upgrading**

Upgrading Terraform is as easy as changing the version in the config, e.g.:

```
diff --git a/terraform/colony.yaml b/terraform/colony.yaml
index 5725a36d..c0ef720f 100644
--- a/terraform/colony.yaml
+++ b/terraform/colony.yaml
@@ -1,7 +1,7 @@
 ---
 
 terraform:
-  version: 0.10.5
+  version: 0.11.7
 
 modules:
  - name: app
```

Colonist will automatically download the new version when it needs it next.

**Detaching from the remote**

Older versions of Terraform had the ability to disable the remote state, which was useful for performing safe upgrades or migrations.

Colonist restores this ability using the `--detach` command to plan, e.g.:

```
colony plan --detach
```

This will create a session directory with a sandbox containing a copy (hard links) of the Terraform code, along with a local copy of the state file:

```
> ls terraform/.tfcolony/01CGC80C81CJFPFCCM0F1FRKDJ/app/sandbox/core/app/terraform.tfstate
terraform/.tfcolony/01CGC80C81CJFPFCCM0F1FRKDJ/app/sandbox/core/app/terraform.tfstate
```

If you need to test anything, you can change directory within the sandbox without affecting the remote.

**Hooks**

Colonist can run run external commands both at startup or before the execution of a module. If `set_env` is `true`, Colonist will parse command
output for `NAME=value` pairs, and set those as environment values.

This can be useful, for example, when using an `assume-role` script to assume an AWS role that requires MFA authentication. If the script outputs
`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `AWS_SESSION_TOKEN` to standard output, then it can be used as a startup hook by Colonist to
transparently change role before running Terraform.

## Use cases

### Dynamic environments

When running a `terraform plan` or `terraform apply`, you can specify custom variables at the command line (using `-var foo=bar`). This can be used to dynamically deploy to a particular environment, or region, for example.

Colonist allows you to specify these variables at runtime, or filter a set of predefined ones.

In the example configuration above, the "app" and "database" modules are deployed to two different environments ("dev" and "prod") by invoking Terraform with different `-var environment=<value>` flags set.

What is happening behind the scenes is the module configuration generates a list of "executions", which is a Cartesian product of each set of possible variable values, plus the user-provided values at run time.

Each execution is then run in parallel, taking into considerations dependencies that modules may have on one another.

### Targeted deploys

Given a list of predefined environments, the user can "filter" which executions are run. For example, the following would run only the executions with enviroment=dev:

```
colony plan --enviroment dev
```

The result would be:

```
> colony plan --region us-east-1 --environment dev
vpc-dev-us-east-1: OK No changes (31s)
database-dev-us-east-1: OK No changes (9s)
app-dev-us-east-1: OK No changes (10s)
> 
```
