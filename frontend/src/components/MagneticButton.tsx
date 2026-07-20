import { useRef, type MouseEvent, type PropsWithChildren } from 'react';
import { motion, useMotionValue, useSpring } from 'framer-motion';

interface MagneticButtonProps extends PropsWithChildren {
  type?: 'button' | 'submit';
  variant?: 'primary' | 'quiet';
  disabled?: boolean;
  className?: string;
  onClick?: () => void;
}

export function MagneticButton({
  children,
  type = 'button',
  variant = 'primary',
  disabled,
  className = '',
  onClick,
}: MagneticButtonProps) {
  const ref = useRef<HTMLButtonElement>(null);
  const x = useSpring(useMotionValue(0), { stiffness: 120, damping: 18 });
  const y = useSpring(useMotionValue(0), { stiffness: 120, damping: 18 });

  function move(event: MouseEvent<HTMLButtonElement>) {
    if (!ref.current || disabled) return;
    const rect = ref.current.getBoundingClientRect();
    x.set((event.clientX - rect.left - rect.width / 2) * 0.12);
    y.set((event.clientY - rect.top - rect.height / 2) * 0.12);
  }

  function reset() {
    x.set(0);
    y.set(0);
  }

  return (
    <motion.button
      ref={ref}
      type={type}
      style={{ x, y }}
      whileHover={disabled ? undefined : { scale: 1.02 }}
      whileTap={disabled ? undefined : { scale: 0.96, y: 2 }}
      transition={{ type: 'spring', stiffness: 420, damping: 28 }}
      onMouseMove={move}
      onMouseLeave={reset}
      onClick={onClick}
      disabled={disabled}
      className={`button ${variant === 'quiet' ? 'button-quiet' : 'button-primary'} ${className}`}
    >
      {children}
    </motion.button>
  );
}
