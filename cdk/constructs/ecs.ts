export interface PortMapping {
  readonly containerPort: number;
  readonly hostPort?: number;
  readonly protocol?: "tcp" | "udp";
}

export interface HealthCheck {
  readonly command?: string[];
  readonly interval?: number;
  readonly timeout?: number;
  readonly retries?: number;
  readonly startPeriod?: number;
}

export interface EnvironmentVariable {
  readonly name: string;
  readonly value: string;
}

export interface AwsLogsOptions {
  readonly "awslogs-group": string;
  readonly "awslogs-region": string;
  readonly "awslogs-stream-prefix": string;
}

export interface LogConfiguration {
  readonly logDriver: "awslogs";
  readonly options: AwsLogsOptions;
}

export interface FargateContainerDefinitionOptions {
  readonly image: string;
  readonly memory?: number;
  readonly memoryReservation?: number;
  readonly portMappings?: PortMapping[];
  readonly healthCheck?: HealthCheck;
  readonly cpu?: number;
  readonly essential?: boolean;
  readonly command?: string[];
  readonly environment?: EnvironmentVariable[];
  readonly logConfiguration?: LogConfiguration;
}

export class FargateContainerDefinition {
  public readonly name: string;
  private readonly options: FargateContainerDefinitionOptions;
  private readonly portMappings: PortMapping[];
  private readonly environments: EnvironmentVariable[];

  constructor(name: string, options: FargateContainerDefinitionOptions) {
    this.name = name;
    this.options = options;
    this.portMappings = [...(options.portMappings ? options.portMappings : [])];
    this.environments = [...(options.environment ? options.environment : [])];
  }

  addPortMapping(...mappings: PortMapping[]): void {
    this.portMappings.push(...mappings);
  }

  addEnvironment(...envs: EnvironmentVariable[]): void {
    this.environments.push(...envs);
  }

  toJSON(): unknown {
    return {
      ...this.options,
      name: this.name,
      ...(this.portMappings.length > 0
        ? { portMappings: this.portMappings }
        : {}),
      ...(this.environments.length > 0
        ? { environment: this.environments }
        : {}),
    };
  }
}
