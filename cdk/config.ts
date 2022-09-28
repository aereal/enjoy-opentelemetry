const keys = ["APP_IMAGE_TAG", "APP_VPC_ID", "APP_SUBNET_IDS"] as const;
const [keyImageTag, keyVpcID, keySubnetIDs] = keys;
type Key = typeof keys[number];

export interface A {
  readonly imageTag: string;
  readonly vpcID: string;
  readonly subnetIDs: string[];
}

interface Env {
  [key: string]: string | undefined;
}

export const consumeEnv = (env: Env): A | Error => {
  const invalidKeys: Key[] = [];
  const imageTag = env[keyImageTag];
  if (imageTag === undefined || imageTag === "") {
    invalidKeys.push(keyImageTag);
  }
  const vpcID = env[keyVpcID];
  if (vpcID === undefined || imageTag === "") {
    invalidKeys.push(keyVpcID);
  }
  const subnetIDList = env[keySubnetIDs];
  if (subnetIDList === undefined || subnetIDList === "") {
    invalidKeys.push(keySubnetIDs);
  }
  if (invalidKeys.length > 0) {
    return new Error(
      `environment variable(s) not defined: ${invalidKeys.join(", ")}`
    );
  }
  return {
    imageTag: imageTag!,
    vpcID: vpcID!,
    subnetIDs: subnetIDList!.split(","),
  };
};
