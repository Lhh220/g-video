import React, { useState } from 'react';
import axios from 'axios';

const EditProfileModal = ({ isOpen, user, onClose, onRefresh }) => {
  const [newUsername, setNewUsername] = useState(user?.username || '');
  const [newAvatar, setNewAvatar] = useState(null);
  const [loading, setLoading] = useState(false);

  if (!isOpen) return null;

  const handleUpdate = async () => {
    const token = localStorage.getItem('token');
    const formData = new FormData();
    formData.append('username', newUsername);
    if (newAvatar) formData.append('avatar', newAvatar);

    setLoading(true);
    try {
      const res = await axios.post('/api/v1/user/update', formData, {
        headers: { 
          'Content-Type': 'multipart/form-data',
          'Authorization': `Bearer ${token}` 
        }
      });
      if (res.data.status_msg === "修改成功") {
        alert("资料修改成功！");
        onRefresh(); // 成功后刷新父页面数据
        onClose();
      }
    // eslint-disable-next-line no-unused-vars
    } catch (err) {
      alert("更新失败，请重试");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[1000] bg-black/90 flex items-center justify-center backdrop-blur-md">
      <div className="bg-zinc-900 w-[400px] p-8 rounded-3xl border border-white/10 shadow-2xl">
        <h2 className="text-white text-xl font-bold mb-8">编辑个人资料</h2>
        <div className="space-y-6">
          <div>
            <label className="text-zinc-500 text-xs mb-2 block">用户名</label>
            <input 
              type="text"
              value={newUsername}
              onChange={(e) => setNewUsername(e.target.value)}
              className="w-full bg-zinc-800 border border-white/5 rounded-xl p-3 text-white outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label className="text-zinc-500 text-xs mb-2 block">新头像</label>
            <input 
              type="file" 
              accept="image/*"
              onChange={(e) => setNewAvatar(e.target.files[0])}
              className="text-sm text-zinc-400"
            />
          </div>
        </div>
        <div className="flex gap-4 mt-10">
          <button onClick={onClose} className="flex-1 py-3 text-zinc-400">取消</button>
          <button 
            onClick={handleUpdate}
            disabled={loading}
            className="flex-1 py-3 bg-white text-black font-bold rounded-xl hover:bg-zinc-200 transition"
          >
            {loading ? '正在保存...' : '保存修改'}
          </button>
        </div>
      </div>
    </div>
  );
};

export default EditProfileModal;