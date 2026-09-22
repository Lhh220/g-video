#!/usr/bin/env python3
# 生成压测用种子数据 SQL：写到 stdout，用法：
#   python gen_seed.py <bcrypt哈希> > seed.sql
#   docker exec -i g-video-mysql mysql -uroot -p123456 g_video < seed.sql
# 幂等：先 DELETE 再 INSERT（只动 id>=100000 的压测数据，不碰真实数据）
import sys
from datetime import datetime, timedelta

if len(sys.argv) < 2:
    sys.stderr.write("用法: gen_seed.py <bench用户密码的bcrypt哈希>\n")
    sys.exit(1)

PWD_HASH = sys.argv[1].replace("'", "\\'")  # 防注入转义

BENCH_UID = 100000          # 压测用户
AUTHOR_IDS = list(range(1, 251))        # 250 个作者
FOLLOWED = AUTHOR_IDS[:50]              # 压测用户关注前 50 个
base = datetime(2026, 9, 1)

def esc(s):
    return s.replace("'", "\\'")

out = []
w = out.append
w("SET NAMES utf8mb4;")
w(f"DELETE FROM likes WHERE user_id={BENCH_UID};")
w(f"DELETE FROM follows WHERE user_id={BENCH_UID};")
w(f"DELETE FROM videos WHERE author_id>={AUTHOR_IDS[0]} AND author_id<={AUTHOR_IDS[-1]};")
w(f"DELETE FROM users WHERE id>={AUTHOR_IDS[0]} AND id<={BENCH_UID};")

# 1. 作者用户 (250) + 压测用户
vals = []
for uid in AUTHOR_IDS:
    vals.append(f"({uid},'author{uid:03d}','$2a$10$placeholderhashxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx','',0)")
vals.append(f"({BENCH_UID},'bench_user','{PWD_HASH}','',0)")
w("INSERT INTO users (id,username,password,avatar,role) VALUES\n  " + ",\n  ".join(vals) + ";")

# 2. 视频：关注作者 50×6=300 条 + 其余作者 200×24=4800 条；再混 200 条待审
vvals = []
vid = 0
now = datetime.now()

def add_video(author, status, dt):
    global vid
    vid += 1
    url = f"https://g-video-assets.oss-cn-wuhan-lr.aliyuncs.com/videos/seed_{vid:06d}.mp4"
    cover = url + "?x-oss-process=video/snapshot,t_1000,f_jpg"
    md5 = f"{vid:032x}"
    vvals.append(f"({vid},{author},'seed video {vid}','{url}','{cover}','{md5}',0,0,{status},'{dt.strftime('%Y-%m-%d %H:%M:%S.%f')[:-3]}')")

# 关注作者的视频：最近的 (关注流主力)
for a in FOLLOWED:
    for k in range(6):
        add_video(a, 1, now - timedelta(minutes=vid * 3))
# 其余作者：分散在过去 20 天
for a in AUTHOR_IDS[50:]:
    for k in range(24):
        add_video(a, 1, base + timedelta(hours=vid % 480))
# 待审视频
for i in range(200):
    add_video(AUTHOR_IDS[i % 200], 0, now - timedelta(minutes=10))
w("INSERT INTO videos (id,author_id,title,play_url,cover_url,file_md5,favorite_count,comment_count,status,created_at) VALUES\n  " + ",\n  ".join(vvals) + ";")

# 3. 关注关系：bench -> 前 50 个作者
w(f"INSERT INTO follows (user_id,to_user_id,created_at) VALUES\n  " +
  ",\n  ".join(f"({BENCH_UID},{a},NOW())" for a in FOLLOWED) + ";")

# 4. 点赞：bench 给关注流里的前 100 个视频点赞
w(f"INSERT INTO likes (user_id,video_id,created_at) VALUES\n  " +
  ",\n  ".join(f"({BENCH_UID},{v},NOW())" for v in range(1, 101)) + ";")

w(f"-- 种子数据: 用户 251, 视频 {vid} (关注流 300 + 其他 4800 + 待审 200), 关注 50, 点赞 100")
sys.stdout.write("\n".join(out) + "\n")
