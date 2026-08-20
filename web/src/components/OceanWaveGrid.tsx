import { useEffect, useRef } from 'react';

/**
 * 3D Undulating Ocean Wave Grid Canvas
 * Simulates gentle, rolling ocean waves undulating across a cyber-mesh grid from bottom to top.
 */
export function OceanWaveGrid() {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext('2d', { alpha: true });
    if (!ctx) return;

    let animationFrameId: number;
    let width = (canvas.width = window.innerWidth);
    let height = (canvas.height = window.innerHeight);
    let time = 0;

    const handleResize = () => {
      if (!canvas) return;
      width = canvas.width = window.innerWidth;
      height = canvas.height = window.innerHeight;
    };

    window.addEventListener('resize', handleResize);

    // Number of columns and rows in the wave grid mesh (Denser & Finer)
    const cols = 56;
    const rows = 40;

    const render = () => {
      time += 0.011; // Gentle, relaxing ocean wave speed

      ctx.clearRect(0, 0, width, height);

      // Grid cell dimensions
      const cellW = width / (cols - 1);
      const cellH = (height * 1.1) / (rows - 1);

      // Compute 3D undulating grid points (x, y, z)
      const points: { x: number; y: number; z: number }[][] = [];

      for (let r = 0; r < rows; r++) {
        points[r] = [];
        for (let c = 0; c < cols; c++) {
          const baseX = c * cellW;
          const baseY = r * cellH - height * 0.05;

          // Rolling ocean swells running from bottom to top
          const wave1 = Math.sin(c * 0.16 + time * 1.3) * 15;
          const wave2 = Math.cos(r * 0.20 - time * 1.7) * 22; // Moving upwards
          const wave3 = Math.sin((c * 0.12 + r * 0.14) - time * 1.1) * 12;
          const waveSwell = Math.sin(c * 0.08 + r * 0.06 - time * 0.5) * 16;

          const z = wave1 + wave2 + wave3 + waveSwell;
          const y = baseY + z;

          points[r][c] = { x: baseX, y, z };
        }
      }

      ctx.lineWidth = 0.6;

      // 1. Draw horizontal undulating ocean wave lines (Soft & Subtle)
      for (let r = 0; r < rows; r++) {
        ctx.beginPath();
        // Soft ambient depth opacity
        const depthAlpha = Math.min(0.09, Math.max(0.015, (r / rows) * 0.08));
        
        const grad = ctx.createLinearGradient(0, 0, width, 0);
        grad.addColorStop(0, `rgba(139, 92, 246, ${depthAlpha * 0.3})`);
        grad.addColorStop(0.3, `rgba(168, 85, 247, ${depthAlpha * 0.9})`);
        grad.addColorStop(0.5, `rgba(0, 210, 255, ${depthAlpha * 1.1})`);
        grad.addColorStop(0.7, `rgba(192, 132, 252, ${depthAlpha * 0.9})`);
        grad.addColorStop(1, `rgba(139, 92, 246, ${depthAlpha * 0.3})`);
        ctx.strokeStyle = grad;

        for (let c = 0; c < cols; c++) {
          const pt = points[r][c];
          if (c === 0) {
            ctx.moveTo(pt.x, pt.y);
          } else {
            const prev = points[r][c - 1];
            const xc = (prev.x + pt.x) / 2;
            const yc = (prev.y + pt.y) / 2;
            ctx.quadraticCurveTo(prev.x, prev.y, xc, yc);
          }
        }
        const lastPt = points[r][cols - 1];
        ctx.lineTo(lastPt.x, lastPt.y);
        ctx.stroke();
      }

      // 2. Draw vertical connecting wave lines (Soft & Subtle)
      for (let c = 0; c < cols; c++) {
        ctx.beginPath();
        const gradV = ctx.createLinearGradient(0, 0, 0, height);
        gradV.addColorStop(0, 'rgba(139, 92, 246, 0.01)');
        gradV.addColorStop(0.5, 'rgba(168, 85, 247, 0.045)');
        gradV.addColorStop(1, 'rgba(0, 210, 255, 0.075)');
        ctx.strokeStyle = gradV;

        for (let r = 0; r < rows; r++) {
          const pt = points[r][c];
          if (r === 0) {
            ctx.moveTo(pt.x, pt.y);
          } else {
            const prev = points[r - 1][c];
            const xc = (prev.x + pt.x) / 2;
            const yc = (prev.y + pt.y) / 2;
            ctx.quadraticCurveTo(prev.x, prev.y, xc, yc);
          }
        }
        const lastPt = points[rows - 1][c];
        ctx.lineTo(lastPt.x, lastPt.y);
        ctx.stroke();
      }

      // 3. Draw gentle shimmering wave crest nodes on peaks (Subtle)
      for (let r = 0; r < rows; r += 2) {
        for (let c = 0; c < cols; c += 2) {
          const pt = points[r][c];
          if (pt.z > 24) { // Crest of wave
            const nodeAlpha = Math.min(0.18, (pt.z - 24) / 35);
            ctx.fillStyle = `rgba(0, 210, 255, ${nodeAlpha})`;
            ctx.beginPath();
            ctx.arc(pt.x, pt.y, 1, 0, Math.PI * 2);
            ctx.fill();
          }
        }
      }

      animationFrameId = requestAnimationFrame(render);
    };

    render();

    return () => {
      window.removeEventListener('resize', handleResize);
      cancelAnimationFrame(animationFrameId);
    };
  }, []);

  return (
    <canvas
      ref={canvasRef}
      className="ocean-wave-canvas"
      aria-hidden="true"
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        width: '100vw',
        height: '100vh',
        pointerEvents: 'none',
        zIndex: 0,
        opacity: 0.45,
      }}
    />
  );
}
