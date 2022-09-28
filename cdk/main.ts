import { App } from "cdktf";
import { consumeEnv } from "./config";
import { MainStack } from "./stacks/main";

const configOrError = consumeEnv(process.env);
if (configOrError instanceof Error) {
  throw configOrError;
}

const app = new App();
new MainStack(app, "enjoy-otel-main", {
  appName: "enjoy-otel",
  awsRegion: "ap-northeast-1",
  imageTags: {
    upstream: configOrError.imageTag,
    downstream: configOrError.imageTag,
    collector: configOrError.imageTag,
  },
  networkConfig: {
    vpcID: configOrError.vpcID,
    subnetIDs: configOrError.subnetIDs,
  },
});
app.synth();
