import { iam } from "@cdktf/provider-aws";
import { Construct } from "constructs";

export interface RoleWithPolicyConfig {
  readonly name: string;
  readonly roleConfig: iam.IamRoleConfig;
  readonly policyDocumentConfig: iam.DataAwsIamPolicyDocumentConfig;
}

export class RoleWithPolicy extends Construct {
  private readonly role: iam.IamRole;

  constructor(scope: Construct, id: string, config: RoleWithPolicyConfig) {
    super(scope, id);
    const { name, roleConfig, policyDocumentConfig } = config;
    this.role = new iam.IamRole(this, "role", roleConfig);
    const policyDoc = new iam.DataAwsIamPolicyDocument(
      this,
      id,
      policyDocumentConfig
    );
    const policy = new iam.IamPolicy(this, "policy", {
      name,
      policy: policyDoc.json,
    });
    new iam.IamRolePolicyAttachment(this, "policy-attatchment", {
      role: this.role.name,
      policyArn: policy.arn,
    });
  }

  get roleArn(): string {
    return this.role.arn;
  }
}
