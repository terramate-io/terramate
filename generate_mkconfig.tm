// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

generate_file "makefiles/_mkconfig.mk" {
  inherit = false

  content = <<EOF
# Copyright 2024 Terramate GmbH
# SPDX-License-Identifier: MPL-2.0

%{for name, val in global.tools~}
${name} ?= ${val}
%{endfor~}
EOF
}