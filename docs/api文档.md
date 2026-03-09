# 接口定义文档

## 一、 g-video RESTful API 全局一览表

统一前缀：api/v1

| **模块** | **功能说明**  | **谓词 (Method)** | **接口路径 (Path)** | **鉴权** | **备注**              |
| -------- | ------------- | ----------------- | ------------------- | -------- | --------------------- |
| **用户** | 注册新用户    | `POST`            | `/user/register`    | ❌        | 返回 Token            |
|          | 用户登录      | `POST`            | `/user/login`       | ❌        | 返回 Token            |
|          | 获取用户信息  | `GET`             | `/user/info`        | ✅        | 获取基础资料          |
|          | 更新用户信息  | `PUT`             | `/user/update/:id`  | ✅        | 更新个人用户信息      |
| **视频** | 发布视频      | `POST`            | `/video/publish`    | ✅        | `multipart/form-data` |
|          | 视频 Feed 流  | `GET`             | `/video/feed`       | ❌/✅      | 游客/登录均可         |
|          | 删除视频      | `DELETE`          | `/video/:id`        | ✅        | DB+OSS同步删          |
| **社交** | 点赞/取消点赞 | `POST`            | `/favorite/action`  | ✅        | 1-点赞，2-取消        |
|          | 关注/取关     | `POST`            | `/relation/action`  | ✅        | 1-关注，2-取关        |
|          | 发表评论      | `POST`            | `/comment/action`   | ✅        | 包含评论内容          |
|          | 删除评论      | `DELETE`          | `/comment/:id`      | ✅        | 仅本人可删            |
| **审核** | 管理员审核    | `POST`            | `/admin/audit`      | ✅        | **限 role=1 权限**    |

## 二、用户模块

### 2.1 用户注册 (Register)

- **Path:** `POST /user/register`
- **Request:** { "username": "admin", "password": "123" }
- **Response:** 包含 `user_id` 和 `token`。

### 2.2 用户登录 (Login)

- **Path:** `POST /user/login`
- **Response:** 同注册，验证成功后颁发新 Token。

### 2.3 获取个人信息 (GetUserInfo)

- **Path:** `GET /user/info`

- **Query:** `user_id=123`

- **Response:**

  ```
  {
    "status_code": 0,
    "user": {
      "id": 1,
      "username": "张三",
      "avatar": "http://oss/avatar.jpg",
      "role": 0, // 0-用户, 1-管理员
      "follower_count": 100,
      "follow_count": 50
    }
  }
  ```

### 2.4 更新用户信息 (UpdateUserInfo)

- **Path:** `PUT /user/update`
- **Content-Type:** `multipart/form-data`
- **Params:** `username` (string, 可选), `avatar` (file, 可选)
- **Logic:** 允许只修改昵称或只换头像。头像上传至 OSS 后，更新数据库 `avatar` 字段。

### 三、视频模块

### 3.1 发布视频 (Publish)

- **Path:** `POST /video/publish`
- **Content-Type:** `multipart/form-data`
- **Params:** `data` (file, 视频文件), `title` (string, 标题)
- **Logic:** 上传 OSS 成功后，MySQL 插入记录，状态 `status` 默认为 0（待审核）。

### 3.2 视频流 (Feed)

- **Path:** `GET /video/feed`
- **Query:** `latest_time` (可选，限制返回视频的时间戳)
- **Response:** 返回 `video_list` 数组，仅包含 `status=1` (已审核通过) 的视频。

### 3.3 删除视频 (Delete)

- **Path:** `DELETE /video/:id`
- **Logic:** 校验是否为视频作者。同步删除 OSS 存储文件，随后删除 MySQL 记录。

## 四、社交模块

### 4.1 点赞操作 (Favorite)

- **Path:** `POST /favorite/action`

- **Request:**

  JSON

  ```
  {
    "video_id": 1001,
    "action_type": 1 // 1-点赞, 2-取消点赞
  }
  ```

- **Logic:** 操作 `likes` 表。成功后通过事务或异步方式更新 `videos` 表的 `favorite_count`。

### 4.2 评论操作 (Comment)

- **Path:** `POST /comment/action`
- **Request:**  { "video_id": 1001, "action_type": 1, // 1-发布, 2-删除 "comment_text": "厉害！", // 仅发布需要 "comment_id": 123 // 仅删除需要 }

### 4.3 关注操作 (Relation)

- **Path:** `POST /relation/action`
- **Request:** `{ "to_user_id": 2, "action_type": 1 }`

## 五、管理员审核模块

### 5.1 视频审核 (Audit)

- **Path:** `POST /admin/audit`

- **Auth:** ✅ 严格校验 JWT 中的 `role == 1`

- **Request:**

  JSON

  ```
  {
    "video_id": 1001,
    "action": 1, // 1-通过, 2-驳回
    "reason": "内容违规" // 仅驳回可选
  }
  ```

- **Logic:** 更新 `videos.status`；若通过，视频即可在 Feed 流中被搜到；若驳回，视频对外部不可见。

