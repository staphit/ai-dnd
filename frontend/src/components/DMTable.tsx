import { useEffect, useRef, useState } from 'react';
import { Canvas, useFrame } from '@react-three/fiber';
import { ContactShadows } from '@react-three/drei';
import * as THREE from 'three';

// DMTable renders a small three.js vignette: a round game table with a seated
// old-man Dungeon Master, built entirely from primitives (no external assets).
// The figure reacts to game state — pondering while the AI deliberates
// (`thinking`), a talking head-bob while narration is read aloud (`speaking`),
// a forward lean in combat, and idle breathing otherwise.
//
// The whole module is imported lazily by App, so three.js lands in its own async
// chunk and never touches the jsdom test run.
interface DMTableProps {
  speaking: boolean;
  thinking: boolean;
  combatActive: boolean;
  scene?: string;
}

// Palette mirrored from the CSS design tokens (src/styles.css :root).
const COLOR = {
  bg: '#11110f',
  surface: '#1d1d19',
  wood: '#23231e',
  accent: '#c59656',
  accentSoft: '#9b7645',
  danger: '#b46e5d',
  skin: '#c9a98a',
  beard: '#d8d3c8',
  dark: '#0c0c0a',
};

const BASE_HEAD_Y = 0.95;

function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    if (typeof window.matchMedia !== 'function') return;
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
    const update = () => setReduced(mq.matches);
    update();
    mq.addEventListener('change', update);
    return () => mq.removeEventListener('change', update);
  }, []);
  return reduced;
}

function Lights({ combatActive }: { combatActive: boolean }) {
  return (
    <>
      <ambientLight intensity={0.35} color={COLOR.bg} />
      <spotLight position={[2.5, 4, 3]} angle={0.5} penumbra={0.8} intensity={22} color={COLOR.accent} castShadow />
      <pointLight position={[-2.5, 1.5, -2]} intensity={6} color={COLOR.accentSoft} />
      <pointLight position={[0, 0.5, 1.1]} intensity={combatActive ? 6 : 3.4} color={combatActive ? COLOR.danger : COLOR.accent} />
    </>
  );
}

function GameTable() {
  return (
    <group>
      {/* wooden top */}
      <mesh position={[0, -0.19, 0]} receiveShadow castShadow>
        <cylinderGeometry args={[1.3, 1.3, 0.08, 48]} />
        <meshStandardMaterial color={COLOR.wood} roughness={0.85} metalness={0.05} />
      </mesh>
      {/* felt surface */}
      <mesh position={[0, -0.14, 0]} receiveShadow>
        <cylinderGeometry args={[1.18, 1.18, 0.02, 48]} />
        <meshStandardMaterial color={COLOR.surface} roughness={0.95} />
      </mesh>
      {/* accent rim */}
      <mesh position={[0, -0.13, 0]}>
        <torusGeometry args={[1.18, 0.02, 12, 48]} />
        <meshStandardMaterial color={COLOR.accentSoft} emissive={COLOR.accentSoft} emissiveIntensity={0.15} roughness={0.6} />
      </mesh>
      {/* legs */}
      {[
        [0.9, -0.7, 0.6],
        [-0.9, -0.7, 0.6],
        [0.9, -0.7, -0.6],
        [-0.9, -0.7, -0.6],
      ].map(([x, y, z], i) => (
        <mesh key={i} position={[x, y, z]} castShadow>
          <cylinderGeometry args={[0.05, 0.05, 1, 12]} />
          <meshStandardMaterial color={COLOR.dark} roughness={0.9} />
        </mesh>
      ))}
    </group>
  );
}

function DiceCluster() {
  return (
    <group position={[0.45, -0.11, 0.55]}>
      <mesh position={[0, 0.03, 0]} rotation={[0.4, 0.6, 0.1]} castShadow>
        <boxGeometry args={[0.09, 0.09, 0.09]} />
        <meshStandardMaterial color={COLOR.accent} emissive={COLOR.accent} emissiveIntensity={0.12} roughness={0.4} />
      </mesh>
      <mesh position={[0.16, 0.03, 0.08]} rotation={[0.2, 0.9, 0.3]} castShadow>
        <boxGeometry args={[0.08, 0.08, 0.08]} />
        <meshStandardMaterial color={COLOR.surface} roughness={0.5} />
      </mesh>
      <mesh position={[-0.14, 0.04, 0.12]} rotation={[0.5, 0.2, 0.7]} castShadow>
        <icosahedronGeometry args={[0.07, 0]} />
        <meshStandardMaterial color={COLOR.accentSoft} emissive={COLOR.accent} emissiveIntensity={0.18} roughness={0.35} metalness={0.2} />
      </mesh>
    </group>
  );
}

function DungeonMaster({ speaking, thinking, combatActive, reduced }: DMTableProps & { reduced: boolean }) {
  const groupRef = useRef<THREE.Group>(null);
  const headRef = useRef<THREE.Group>(null);
  const beardRef = useRef<THREE.Mesh>(null);
  const armRef = useRef<THREE.Group>(null);

  useFrame((state, delta) => {
    const g = groupRef.current;
    const head = headRef.current;
    const beard = beardRef.current;
    const arm = armRef.current;
    if (!g || !head || !beard || !arm) return;

    if (reduced) {
      // Freeze to a calm, upright resting pose.
      g.scale.y = 1;
      g.rotation.x = combatActive ? 0.12 : 0;
      head.position.y = BASE_HEAD_Y;
      head.rotation.z = thinking ? 0.18 : 0;
      beard.scale.y = 1;
      arm.rotation.z = thinking ? -0.9 : -0.2;
      return;
    }

    const t = state.clock.elapsedTime;
    const M = THREE.MathUtils;
    // idle breathing
    g.scale.y = M.lerp(g.scale.y, 1 + Math.sin(t * 1.6) * 0.012, 0.15);
    // combat: lean forward toward the table
    g.rotation.x = M.damp(g.rotation.x, combatActive ? 0.12 : 0, 3, delta);
    // speaking: quick head bob + beard/jaw oscillation
    const talkBob = speaking ? Math.sin(t * 11) * 0.05 : 0;
    head.position.y = M.damp(head.position.y, BASE_HEAD_Y + talkBob, 8, delta);
    beard.scale.y = M.damp(beard.scale.y, speaking ? 1 + Math.abs(Math.sin(t * 11)) * 0.16 : 1, 8, delta);
    // thinking: tilt head, raise hand toward chin
    head.rotation.z = M.damp(head.rotation.z, thinking ? 0.18 : 0, 4, delta);
    arm.rotation.z = M.damp(arm.rotation.z, thinking ? -0.95 : -0.2, 4, delta);
  });

  return (
    <group ref={groupRef} position={[0, -0.15, -0.4]}>
      {/* cloak / body */}
      <mesh position={[0, 0.32, 0]} castShadow>
        <cylinderGeometry args={[0.3, 0.52, 0.9, 24]} />
        <meshStandardMaterial color={COLOR.surface} roughness={0.9} />
      </mesh>
      {/* shoulder collar accent */}
      <mesh position={[0, 0.72, 0]}>
        <cylinderGeometry args={[0.31, 0.31, 0.06, 24]} />
        <meshStandardMaterial color={COLOR.accentSoft} roughness={0.7} />
      </mesh>

      {/* head group (bobs / tilts) */}
      <group ref={headRef} position={[0, BASE_HEAD_Y, 0.02]}>
        <mesh castShadow>
          <sphereGeometry args={[0.2, 24, 24]} />
          <meshStandardMaterial color={COLOR.skin} roughness={0.7} />
        </mesh>
        {/* eyes */}
        <mesh position={[0.07, 0.03, 0.17]}>
          <sphereGeometry args={[0.025, 10, 10]} />
          <meshStandardMaterial color={COLOR.dark} />
        </mesh>
        <mesh position={[-0.07, 0.03, 0.17]}>
          <sphereGeometry args={[0.025, 10, 10]} />
          <meshStandardMaterial color={COLOR.dark} />
        </mesh>
        {/* beard (points down, jaw oscillates) */}
        <mesh ref={beardRef} position={[0, -0.2, 0.12]} rotation={[Math.PI, 0, 0]}>
          <coneGeometry args={[0.15, 0.34, 16]} />
          <meshStandardMaterial color={COLOR.beard} roughness={1} />
        </mesh>
        {/* wizard hat brim */}
        <mesh position={[0, 0.16, 0]}>
          <cylinderGeometry args={[0.3, 0.3, 0.04, 24]} />
          <meshStandardMaterial color={COLOR.accentSoft} roughness={0.75} />
        </mesh>
        {/* wizard hat cone */}
        <mesh position={[0, 0.42, 0]} castShadow>
          <coneGeometry args={[0.24, 0.5, 24]} />
          <meshStandardMaterial color={COLOR.accent} roughness={0.65} emissive={COLOR.accentSoft} emissiveIntensity={0.08} />
        </mesh>
      </group>

      {/* left arm resting on table */}
      <group position={[-0.34, 0.62, 0.08]} rotation={[0, 0, 0.35]}>
        <mesh position={[0, -0.2, 0]} castShadow>
          <cylinderGeometry args={[0.08, 0.09, 0.5, 14]} />
          <meshStandardMaterial color={COLOR.surface} roughness={0.9} />
        </mesh>
        <mesh position={[0, -0.46, 0.02]}>
          <sphereGeometry args={[0.08, 14, 14]} />
          <meshStandardMaterial color={COLOR.skin} roughness={0.7} />
        </mesh>
      </group>

      {/* right arm (raises toward chin when thinking) */}
      <group ref={armRef} position={[0.32, 0.64, 0.08]} rotation={[0, 0, -0.2]}>
        <mesh position={[0, -0.2, 0]} castShadow>
          <cylinderGeometry args={[0.08, 0.09, 0.5, 14]} />
          <meshStandardMaterial color={COLOR.surface} roughness={0.9} />
        </mesh>
        <mesh position={[0, -0.46, 0.02]}>
          <sphereGeometry args={[0.08, 14, 14]} />
          <meshStandardMaterial color={COLOR.skin} roughness={0.7} />
        </mesh>
      </group>
    </group>
  );
}

export function DMTable({ speaking, thinking, combatActive, scene }: DMTableProps) {
  const reduced = usePrefersReducedMotion();
  return (
    <section className="dm-table" aria-label={`地城主的牌桌${scene ? `：${scene}` : ''}`}>
      <Canvas
        shadows
        dpr={[1, 1.75]}
        camera={{ position: [0, 0.55, 3.4], fov: 42 }}
        gl={{ alpha: true, antialias: true }}
        frameloop={reduced ? 'demand' : 'always'}
      >
        <Lights combatActive={combatActive} />
        <GameTable />
        <DiceCluster />
        <DungeonMaster speaking={speaking} thinking={thinking} combatActive={combatActive} reduced={reduced} />
        <ContactShadows position={[0, -0.62, 0]} opacity={0.5} blur={2.4} scale={4} far={2} color="#000000" />
      </Canvas>
    </section>
  );
}
