# g-video 项目数据库设计

## 1. 用户表（users）

存储用户核心账号信息。

| **字段名**   | **类型**     | **约束**         | **默认值**        | **说明**                   |
| ------------ | ------------ | ---------------- | ----------------- | -------------------------- |
| `id`         | BIGINT       | PRIMARY KEY      | 自增              | 用户唯一标识               |
| `username`   | VARCHAR(64)  | UNIQUE, NOT NULL | -                 | 登录账号                   |
| `password`   | VARCHAR(255) | NOT NULL         | -                 | Bcrypt加密后的密文         |
| `avatar`     | VARCHAR(255) | -                | ''                | 头像在OSS中的URL           |
| `created_at` | DATETIME     | -                | CURRENT_TIMESTAMP | 注册时间                   |
| `updated_at` | DATETIME     | -                | CURRENT_TIMESTAMP | 资料更新时间               |
| `role`       | TINYINT      | -                | 0                 | 身份：0-普通用户, 1-管理员 |

## 2. 视频表（videos）

核心表，存储视频元数据及审核状态。

| **字段名**       | **类型**     | **约束**        | **默认值**        | **说明**                       |
| ---------------- | ------------ | --------------- | ----------------- | ------------------------------ |
| `id`             | BIGINT       | PRIMARY KEY     | 自增              | 视频唯一 ID                    |
| `author_id`      | BIGINT       | INDEX, NOT NULL | -                 | 上传者用户 ID                  |
| `title`          | VARCHAR(128) | NOT NULL        | -                 | 视频标题                       |
| `play_url`       | VARCHAR(255) | NOT NULL        | -                 | OSS 存储的 ObjectKey           |
| `cover_url`      | VARCHAR(255) | NOT NULL        | -                 | 封面图存储的 ObjectKey         |
| `status`         | TINYINT      | INDEX           | 0                 | 状态：0-待审核, 1-发布, 2-驳回 |
| `created_at`     | DATETIME     | INDEX           | CURRENT_TIMESTAMP | 投稿时间                       |
| `comment_count`  | BIGINT       | -               | 0                 | 视频评论总数                   |
| `favorite_count` | BIGINT       | -               | 0                 | 视频点赞总数                   |



### 3. 关注关系表(follows)

维护用户间的社交关系。

| **字段名**    | **类型** | **约束**    | **默认值**        | **说明**             |
| ------------- | -------- | ----------- | ----------------- | -------------------- |
| `id`          | BIGINT   | PRIMARY KEY | 自增              | 自增 ID              |
| `user_id`     | BIGINT   | NOT NULL    | -                 | 发起关注者 ID (粉丝) |
| `follower_id` | BIGINT   | NOT NULL    | -                 | 被关注者 ID (博主)   |
| `created_at`  | DATETIME | -           | CURRENT_TIMESTAMP | 关注时间             |

### 4. 审核日志表 (audit_logs)

记录管理员对视频的审核路径。

| **字段名**   | **类型**     | **约束**    | **默认值**        | **说明**          |
| ------------ | ------------ | ----------- | ----------------- | ----------------- |
| `id`         | BIGINT       | PRIMARY KEY | 自增              | 日志 ID           |
| `video_id`   | BIGINT       | NOT NULL    | -                 | 关联的视频 ID     |
| `admin_id`   | BIGINT       | NOT NULL    | -                 | 操作人(管理员) ID |
| `action`     | TINYINT      | NOT NULL    | -                 | 1-通过, 2-驳回    |
| `reason`     | VARCHAR(255) | -           | ''                | 驳回的具体原因    |
| `created_at` | DATETIME     | -           | CURRENT_TIMESTAMP | 操作时间          |

### 5. 点赞表(likes)

维护用户与视频的喜爱关系。

| **字段名**   | **类型** | **约束**                    | **说明**        |
| ------------ | -------- | --------------------------- | --------------- |
| `id`         | BIGINT   | PRIMARY KEY, AUTO_INCREMENT | 自增 ID         |
| `user_id`    | BIGINT   | UNIQUE_KEY_1                | 点赞者 ID       |
| `video_id`   | BIGINT   | UNIQUE_KEY_2, INDEX         | 被点赞的视频 ID |
| `created_at` | DATETIME | -                           | 点赞时间        |

## 6. 评论表(comments)

记录用户对视频的交互内容。

| **字段名**   | **类型** | **约束**                    | **说明**                           |
| ------------ | -------- | --------------------------- | ---------------------------------- |
| `id`         | BIGINT   | PRIMARY KEY, AUTO_INCREMENT | 评论唯一标识                       |
| `user_id`    | BIGINT   | INDEX, NOT NULL             | 发送评论的用户 ID                  |
| `video_id`   | BIGINT   | INDEX, NOT NULL             | 所属视频 ID                        |
| `content`    | TEXT     | NOT NULL                    | 评论文本内容（使用 TEXT 防止超长） |
| `created_at` | DATETIME | -                           | 评论发布时间                       |

