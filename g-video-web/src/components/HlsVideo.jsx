import React, { forwardRef, useEffect } from 'react';
import Hls from 'hls.js';

// 优先播放异步转码生成的 HLS 切片 (hls_url)，没有则回退 mp4 直链
// Safari 原生支持 HLS，其余浏览器通过 hls.js (MSE) 播放
const HlsVideo = forwardRef(({ video, ...rest }, ref) => {
  useEffect(() => {
    const el = ref?.current;
    if (!el) return;

    const hlsURL = video?.hls_url;
    if (!hlsURL || el.canPlayType('application/vnd.apple.mpegurl')) {
      el.src = hlsURL || video?.play_url;
      return;
    }
    if (Hls.isSupported()) {
      const hls = new Hls({ enableWorker: true });
      hls.loadSource(hlsURL);
      hls.attachMedia(el);
      return () => hls.destroy();
    }
    el.src = video?.play_url; // 极端情况兜底
  }, [video?.hls_url, video?.play_url, ref]);

  // src 不作为 prop 传入，由上面的 effect 统一接管，避免与 hls.js 冲突
  return <video ref={ref} {...rest} />;
});

export default HlsVideo;
