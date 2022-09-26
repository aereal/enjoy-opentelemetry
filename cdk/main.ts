import { App } from "cdktf";
import { MainStack } from "./stacks/main";

const app = new App();
new MainStack(app, "enjoy-otel-main");
app.synth();
