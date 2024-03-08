script "test" "preview" {
  description = "Create a Terramate Cloud Preview (e2etests)"
  job {
    command = tm_concat(global.cmd.test.command, [
      "-parallel=1000", // e2etests are highly parallelizable
      {
        cloud_sync_preview             = true,
        cloud_sync_terraform_plan_file = "test.plan"
    }])
  }
}
