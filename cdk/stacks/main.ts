import { TerraformStack } from "cdktf";
import { Construct } from "constructs";

export class MainStack extends TerraformStack {
  constructor(scope: Construct, name: string) {
    super(scope, name);

    // define resources here
  }
}
