# tgdeps - discover Terragrunt Modules and its dependencies

This program can be used to check all files and directories that Terragrunt
modules depend on. It can be invoked inside an specific module directory or
from the project root dir (git repository root or stack directory).

## Install

```
go install github.com/terramate-io/terramate/cmd/tgdeps@<version>
```

or clone the Terramate project and run:

```
$ make install/tgdeps
```

## Usage

Example running in the official [insfrastructure-live for Terragrunt](https://github.com/gruntwork-io/terragrunt-infrastructure-live-example) repository:

```
$ git clone https://github.com/gruntwork-io/terragrunt-infrastructure-live-example.git
$ cd terragrunt-infrastructure-live-example.git
```

and then run `tgdeps`:

```
$ tgdeps
Module: /non-prod/us-east-1/qa/mysql
	- /_envcommon/mysql.hcl
	- /non-prod/account.hcl
	- /non-prod/us-east-1/qa/env.hcl
	- /non-prod/us-east-1/region.hcl
	- /terragrunt.hcl
Module: /non-prod/us-east-1/qa/webserver-cluster
	- /_envcommon/webserver-cluster.hcl
	- /non-prod/account.hcl
	- /non-prod/us-east-1/qa/env.hcl
	- /non-prod/us-east-1/region.hcl
	- /terragrunt.hcl
Module: /non-prod/us-east-1/stage/mysql
	- /_envcommon/mysql.hcl
	- /non-prod/account.hcl
	- /non-prod/us-east-1/region.hcl
	- /non-prod/us-east-1/stage/env.hcl
	- /terragrunt.hcl
Module: /prod/us-east-1/prod/mysql
	- /_envcommon/mysql.hcl
	- /prod/account.hcl
	- /prod/us-east-1/prod/env.hcl
	- /prod/us-east-1/prod/mysql/config.hcl
	- /prod/us-east-1/region.hcl
	- /terragrunt.hcl
```

Alternatively, you can have the output in JSON format:

`$ tgdeps -json`

```json
[
  {
    "path": "/non-prod/us-east-1/qa/mysql",
    "config": "/non-prod/us-east-1/qa/mysql/terragrunt.hcl",
    "depends_on": [
      "/_envcommon/mysql.hcl",
      "/non-prod/account.hcl",
      "/non-prod/us-east-1/qa/env.hcl",
      "/non-prod/us-east-1/region.hcl",
      "/terragrunt.hcl"
    ]
  },
  {
    "path": "/non-prod/us-east-1/qa/webserver-cluster",
    "config": "/non-prod/us-east-1/qa/webserver-cluster/terragrunt.hcl",
    "depends_on": [
      "/_envcommon/webserver-cluster.hcl",
      "/non-prod/account.hcl",
      "/non-prod/us-east-1/qa/env.hcl",
      "/non-prod/us-east-1/region.hcl",
      "/terragrunt.hcl"
    ]
  },
  {
    "path": "/non-prod/us-east-1/stage/mysql",
    "config": "/non-prod/us-east-1/stage/mysql/terragrunt.hcl",
    "depends_on": [
      "/_envcommon/mysql.hcl",
      "/non-prod/account.hcl",
      "/non-prod/us-east-1/region.hcl",
      "/non-prod/us-east-1/stage/env.hcl",
      "/terragrunt.hcl"
    ]
  },
  {
    "path": "/prod/us-east-1/prod/mysql",
    "config": "/prod/us-east-1/prod/mysql/terragrunt.hcl",
    "depends_on": [
      "/_envcommon/mysql.hcl",
      "/prod/account.hcl",
      "/prod/us-east-1/prod/env.hcl",
      "/prod/us-east-1/region.hcl",
      "/terragrunt.hcl"
    ]
  }
]
```
