drop table if exists livers;
create table if not exists livers (
  `liver_id` bigint auto_increment primary key,
  `name` varchar(255) not null unique key,
  `debuted_on` date not null,
  `retired_on` date
) engine=InnoDB default character set=utf8mb4;

drop table if exists `liver_groups`;
create table if not exists `liver_groups` (
  `group_id` bigint unsigned not null auto_increment primary key,
  `name` varchar(255) not null unique key
) engine=InnoDB default charset=utf8mb4;

drop table if exists `liver_group_members`;
create table if not exists `liver_group_members` (
  `liver_group_id` bigint unsigned not null,
  `liver_id` bigint signed not null,
  primary key (`liver_group_id`, `liver_id`)
) engine=InnoDB default charset=utf8mb4;
