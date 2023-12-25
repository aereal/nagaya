create database tenant_1;

use tenant_1;

create table if not exists users (
  id bigint unsigned auto_increment primary key
) ENGINE=INNODB DEFAULT CHARSET=utf8mb4 collate=utf8mb4_unicode_ci;

create database tenant_2;

use tenant_2;

create table if not exists users (
  id bigint unsigned auto_increment primary key
) ENGINE=INNODB DEFAULT CHARSET=utf8mb4 collate=utf8mb4_unicode_ci;

create database tenant_3;

use tenant_3;

create table if not exists users (
  id bigint unsigned auto_increment primary key
) ENGINE=INNODB DEFAULT CHARSET=utf8mb4 collate=utf8mb4_unicode_ci;
