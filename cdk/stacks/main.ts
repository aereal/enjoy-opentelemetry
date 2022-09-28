import {
  AwsProvider,
  cloudwatch,
  ecr,
  ecs,
  iam,
  vpc,
} from "@cdktf/provider-aws";
import { TerraformStack } from "cdktf";
import { Construct } from "constructs";
import { FargateContainerDefinition } from "../constructs/ecs";
import { RoleWithPolicy } from "../constructs/role-with-policy";

type CommonEcrRepositoryConfig = Omit<ecr.EcrRepositoryConfig, "name">;

interface NetworkConfig {
  readonly vpcID: string;
  readonly subnetIDs: string[];
}

interface MainStackConfig {
  readonly appName: string;
  readonly awsRegion: string;
  readonly imageTags: Record<"upstream" | "downstream" | "collector", string>;
  readonly networkConfig: NetworkConfig;
}

export class MainStack extends TerraformStack {
  constructor(scope: Construct, name: string, config: MainStackConfig) {
    super(scope, name);

    const { appName: prefix, awsRegion, imageTags, networkConfig } = config;
    new AwsProvider(this, "aws-provider", {
      region: awsRegion,
    });
    const sg = new vpc.SecurityGroup(this, "app-service-sg", {
      vpcId: networkConfig.vpcID,
    });

    const commonRepoConfig: CommonEcrRepositoryConfig = {
      imageTagMutability: "IMMUTABLE",
      imageScanningConfiguration: {
        scanOnPush: false,
      },
    };

    const upstreamRepo = new ecr.EcrRepository(this, "app-upstream", {
      name: `${prefix}-upstream`,
      ...commonRepoConfig,
    });
    const downstreamRepo = new ecr.EcrRepository(this, "app-downstream", {
      name: `${prefix}-downstream`,
      ...commonRepoConfig,
    });
    const collectorRepo = new ecr.EcrRepository(this, "collector", {
      name: `${prefix}-collector`,
      ...commonRepoConfig,
    });

    const cluster = new ecs.EcsCluster(this, "cluster", {
      name: `${prefix}-app`,
      setting: [{ name: "containerInsights", value: "enabled" }],
    });

    const logGroup = new cloudwatch.CloudwatchLogGroup(this, "log-group", {
      retentionInDays: 3,
      name: `${prefix}-app`,
    });
    AwsProvider.isConstruct;

    const allowFromECSTasks = new iam.DataAwsIamPolicyDocument(
      this,
      "allow-from-ecs-tasks",
      {
        statement: [
          {
            actions: ["sts:AssumeRole"],
            principals: [
              {
                type: "Service",
                identifiers: ["ecs-tasks.amazonaws.com"],
              },
            ],
          },
        ],
      }
    );
    const taskRole = new RoleWithPolicy(this, "app-task-role", {
      name: `${prefix}-app-task`,
      roleConfig: {
        name: `${prefix}-app-task`,
        assumeRolePolicy: allowFromECSTasks.json,
      },
      policyDocumentConfig: {
        statement: [{ actions: ["ecs:List*"], resources: ["*"] }],
      },
    });
    const executionRole = new RoleWithPolicy(this, "app-execution-role", {
      name: `${prefix}-app-execution`,
      roleConfig: {
        name: `${prefix}-app-execution`,
        assumeRolePolicy: allowFromECSTasks.json,
      },
      policyDocumentConfig: {
        statement: [
          {
            actions: [
              "ecr:GetAuthorizationToken",
              "logs:CreateLogStream",
              "logs:PutLogEvents",
            ],
            resources: ["*"],
          },
          {
            actions: [
              "ecr:BatchCheckLayerAvailability",
              "ecr:GetDownloadUrlForLayer",
              "ecr:BatchGetImage",
            ],
            resources: [
              upstreamRepo.arn,
              downstreamRepo.arn,
              collectorRepo.arn,
            ],
          },
        ],
      },
    });

    const upstreamContainerDefinition = new FargateContainerDefinition(
      "upstream",
      {
        image: `${upstreamRepo.repositoryUrl}:${imageTags.upstream}`,
        essential: true,
        logConfiguration: {
          logDriver: "awslogs",
          options: {
            "awslogs-group": logGroup.name,
            "awslogs-region": awsRegion,
            "awslogs-stream-prefix": imageTags.upstream,
          },
        },
      }
    );
    const downstreamContainerDefinition = new FargateContainerDefinition(
      "downstream",
      {
        image: `${downstreamRepo.repositoryUrl}:${imageTags.downstream}`,
        essential: true,
        logConfiguration: {
          logDriver: "awslogs",
          options: {
            "awslogs-group": logGroup.name,
            "awslogs-region": awsRegion,
            "awslogs-stream-prefix": imageTags.downstream,
          },
        },
      }
    );
    const collectorContainerDefinition = new FargateContainerDefinition(
      "collector",
      {
        image: `${collectorRepo.repositoryUrl}:${imageTags.collector}`,
        logConfiguration: {
          logDriver: "awslogs",
          options: {
            "awslogs-group": logGroup.name,
            "awslogs-region": awsRegion,
            "awslogs-stream-prefix": imageTags.collector,
          },
        },
      }
    );
    const td = new ecs.EcsTaskDefinition(this, "app-task-definition", {
      family: "app",
      containerDefinitions: JSON.stringify([
        upstreamContainerDefinition,
        downstreamContainerDefinition,
        collectorContainerDefinition,
      ]),
      executionRoleArn: executionRole.roleArn,
      taskRoleArn: taskRole.roleArn,
      networkMode: "awsvpc",
      requiresCompatibilities: ["FARGATE"],
      cpu: "1024",
      memory: "3072",
    });
    new ecs.EcsService(this, "app-service", {
      cluster: cluster.arn,
      desiredCount: 1,
      launchType: "FARGATE",
      name: prefix,
      platformVersion: "1.4.0",
      schedulingStrategy: "REPLICA",
      taskDefinition: td.arn,
      enableExecuteCommand: true,
      networkConfiguration: {
        assignPublicIp: false,
        securityGroups: [sg.id],
        subnets: networkConfig.subnetIDs,
      },
    });
  }
}
