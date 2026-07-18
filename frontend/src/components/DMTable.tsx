import { useEffect, useMemo, useRef, useState } from 'react';
import { Canvas, useFrame, useThree } from '@react-three/fiber';
import { ContactShadows, OrbitControls, Sparkles, useAnimations, useGLTF } from '@react-three/drei';
import type { OrbitControls as OrbitControlsImpl } from 'three-stdlib';
import * as THREE from 'three';
// Deep clone that rebinds SkinnedMesh → cloned bones (scene.clone does not).
import { clone as cloneSkinned } from 'three/examples/jsm/utils/SkeletonUtils.js';

// DM portrait: table props stay procedural; the figure is swapped between GLB
// clips (idle / talk / dice success / dice fail) from repo `/glb`.

import idleUrl from '../../../glb/idle.glb?url';
import talkUrl from '../../../glb/talk.glb?url';
import diceSuccUrl from '../../../glb/dice_succ.glb?url';
import diceFailUrl from '../../../glb/dice_fail.glb?url';
import {
  clampDmAvatarScale,
  DM_AVATAR_SCALE_DEFAULT,
} from './dmAvatarScale';

export type DiceAnimOutcome = 'success' | 'fail' | null;

export interface DMTableProps {
  speaking: boolean;
  thinking: boolean;
  combatActive: boolean;
  /** True while a required d20 is being resolved. */
  rolling?: boolean;
  /** After the roll is known — drives cheer vs headache clips. */
  rollOutcome?: DiceAnimOutcome;
  /** A check is waiting (glow the dice, ready pose). */
  checkPending?: boolean;
  scene?: string;
  /** Manual figure scale (Mixamo ~1.7u base). Default keeps full body in front of the table. */
  avatarScale?: number;
}

export {
  clampDmAvatarScale,
  DM_AVATAR_SCALE_DEFAULT,
  DM_AVATAR_SCALE_MIN,
  DM_AVATAR_SCALE_MAX,
} from './dmAvatarScale';

type AvatarMode = 'idle' | 'talk' | 'dice_succ' | 'dice_fail';

const GLB = {
  idle: idleUrl,
  talk: talkUrl,
  dice_succ: diceSuccUrl,
  dice_fail: diceFailUrl,
} as const;

const COLOR = {
  void: '#0e0a08',
  voidDeep: '#080604',
  woodDeep: '#241810',
  wood: '#3c2c1e',
  woodLight: '#5c4632',
  felt: '#1a2218',
  brass: '#e0b24e',
  brassDim: '#b8893a',
  brassGlow: '#f0d078',
  danger: '#d06050',
  dangerGlow: '#e88870',
  flame: '#ffb040',
  flameCore: '#fff0c0',
  parchment: '#d0bc98',
  stone: '#2a2420',
  stoneLight: '#3a322c',
  stoneDark: '#161210',
  plaster: '#3c342c',
  curtain: '#3a2018',
};

/**
 * Portrait layout (camera at +z looking toward −z):
 * - Character on floor; game table in front at waist height (Mixamo hips ≈ 0.95).
 * - Drag to orbit/pan, scroll wheel to zoom (OrbitControls).
 */
const LAYOUT = {
  avatar: [0, 0, 0] as [number, number, number],
  /** Local Y of Mixamo hips / waist before avatar scale. */
  hipLocalY: 0.95,
  /** Table sits just in front of the figure (toward camera). */
  tableZ: 0.48,
  tableScale: 0.55,
  cameraPos: [0, 1.05, 3.35] as [number, number, number],
  lookAt: [0, 0.82, 0.15] as [number, number, number],
  fov: 34,
  minDistance: 1.5,
  maxDistance: 8,
};

// Back-compat alias used by GltfFigure outer group.
const AVATAR = { position: LAYOUT.avatar };

/** World placement for the table + props so the top meets the figure's waist. */
function tablePlacement(avatarScale: number) {
  const s = clampDmAvatarScale(avatarScale);
  const waistY = LAYOUT.hipLocalY * s;
  // Felt top is slightly above the group origin after tableScale.
  const topOffset = 0.04 * LAYOUT.tableScale;
  return {
    position: [0, Math.max(0.08, waistY - topOffset), LAYOUT.tableZ] as [number, number, number],
    scale: LAYOUT.tableScale,
    waistY,
  };
}

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

function resolveMode(props: {
  speaking: boolean;
  thinking: boolean;
  rolling: boolean;
  rollOutcome: DiceAnimOutcome;
}): AvatarMode {
  if (props.rolling) {
    if (props.rollOutcome === 'success') return 'dice_succ';
    if (props.rollOutcome === 'fail') return 'dice_fail';
    // Outcome not known yet — stay idle while dice tumble.
    return 'idle';
  }
  if (props.speaking) return 'talk';
  return 'idle';
}

/** Seed camera once; OrbitControls owns it afterwards (drag + wheel). */
function InitialCamera({ lookAt }: { lookAt: [number, number, number] }) {
  const { camera } = useThree();
  const seeded = useRef(false);
  useEffect(() => {
    if (seeded.current) return;
    seeded.current = true;
    const cam = camera as THREE.PerspectiveCamera;
    cam.position.set(...LAYOUT.cameraPos);
    cam.fov = LAYOUT.fov;
    cam.lookAt(...lookAt);
    cam.updateProjectionMatrix();
  }, [camera, lookAt]);
  return null;
}

/** Drag to orbit/pan, scroll wheel to zoom. Double-click resets the view. */
function PortraitControls({ target }: { target: [number, number, number] }) {
  const controls = useRef<OrbitControlsImpl>(null);
  const { camera, gl } = useThree();

  useEffect(() => {
    const el = gl.domElement;
    const onDbl = () => {
      camera.position.set(...LAYOUT.cameraPos);
      if (controls.current) {
        controls.current.target.set(...target);
        controls.current.update();
      } else {
        camera.lookAt(...target);
      }
    };
    el.addEventListener('dblclick', onDbl);
    return () => el.removeEventListener('dblclick', onDbl);
  }, [camera, gl, target]);

  return (
    <OrbitControls
      ref={controls}
      makeDefault
      target={target}
      enableDamping
      dampingFactor={0.12}
      enablePan
      enableZoom
      enableRotate
      minDistance={LAYOUT.minDistance}
      maxDistance={LAYOUT.maxDistance}
      maxPolarAngle={Math.PI * 0.78}
      minPolarAngle={Math.PI * 0.12}
      zoomSpeed={0.75}
      panSpeed={0.7}
      rotateSpeed={0.55}
      // Left = orbit, right = pan, wheel = zoom (three.js defaults).
    />
  );
}

function Lights({ combatActive, rolling, reduced }: { combatActive: boolean; rolling: boolean; reduced: boolean }) {
  const candle = useRef<THREE.PointLight>(null);
  const torchL = useRef<THREE.PointLight>(null);
  const torchR = useRef<THREE.PointLight>(null);
  const flash = useRef<THREE.PointLight>(null);
  useFrame((state) => {
    const t = state.clock.elapsedTime;
    if (!reduced) {
      const f = 1 + Math.sin(t * 8) * 0.1 + Math.sin(t * 17) * 0.05;
      if (candle.current) candle.current.intensity = (rolling ? 1.8 : combatActive ? 2.6 : 2.2) * f;
      if (torchL.current) torchL.current.intensity = 2.4 * (1 + Math.sin(t * 6.2) * 0.12);
      if (torchR.current) torchR.current.intensity = 2.2 * (1 + Math.sin(t * 7.1 + 1.2) * 0.1);
    }
    if (flash.current) {
      flash.current.intensity = rolling ? 3.2 + Math.sin(t * 40) * 1.4 : 0;
    }
  });
  return (
    <>
      <ambientLight intensity={0.22} color="#3a3028" />
      <hemisphereLight args={['#5a4838', '#0a0806', 0.38]} />
      <spotLight
        position={[1.2, 3.2, 2.2]}
        angle={0.55}
        penumbra={0.9}
        intensity={14}
        color={COLOR.brassGlow}
        castShadow
        shadow-mapSize-width={1024}
        shadow-mapSize-height={1024}
        shadow-bias={-0.00025}
      />
      <pointLight position={[-1.4, 1.6, 1.0]} intensity={1.6} color="#5a4030" distance={6} />
      <pointLight
        ref={candle}
        position={[-0.45, 0.55, 0.55]}
        intensity={2.2}
        color={combatActive ? COLOR.dangerGlow : COLOR.flame}
        distance={4}
        decay={2}
      />
      <pointLight ref={torchL} position={[-1.35, 1.55, -0.9]} intensity={2.4} color={COLOR.flame} distance={5} decay={2} />
      <pointLight ref={torchR} position={[1.35, 1.55, -0.9]} intensity={2.2} color={COLOR.flame} distance={5} decay={2} />
      <pointLight ref={flash} position={[0.2, 0.7, 0.6]} intensity={0} color={COLOR.brassGlow} distance={3.5} />
      <fog attach="fog" args={[COLOR.voidDeep, 3.2, 8.5]} />
    </>
  );
}

/** Stone chamber backdrop so the portrait is not a flat void. */
function ChamberBackground({ combatActive }: { combatActive: boolean }) {
  const wallColor = combatActive ? '#2a1814' : COLOR.stone;
  const accent = combatActive ? COLOR.danger : COLOR.brassDim;
  return (
    <group>
      {/* Floor */}
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.01, -0.4]} receiveShadow>
        <circleGeometry args={[4.2, 48]} />
        <meshStandardMaterial color={COLOR.stoneDark} roughness={0.95} metalness={0.05} />
      </mesh>
      {/* Back wall (inward-facing cylinder segment) */}
      <mesh position={[0, 1.15, -1.55]}>
        <cylinderGeometry args={[2.6, 2.6, 2.6, 32, 1, true, Math.PI * 0.22, Math.PI * 1.56]} />
        <meshStandardMaterial color={wallColor} roughness={0.92} metalness={0.04} side={THREE.BackSide} />
      </mesh>
      {/* Rear flat wall for soft fill */}
      <mesh position={[0, 1.1, -2.35]} receiveShadow>
        <planeGeometry args={[5.2, 2.8]} />
        <meshStandardMaterial color={COLOR.stoneDark} roughness={0.96} />
      </mesh>
      {/* Arch band */}
      <mesh position={[0, 2.15, -1.5]} rotation={[Math.PI / 2, 0, 0]}>
        <torusGeometry args={[1.55, 0.05, 8, 40, Math.PI]} />
        <meshStandardMaterial color={accent} metalness={0.55} roughness={0.4} emissive={accent} emissiveIntensity={0.08} />
      </mesh>
      {/* Side pillars */}
      {([-1.15, 1.15] as const).map((x) => (
        <group key={x} position={[x, 0, -1.05]}>
          <mesh position={[0, 1.05, 0]} castShadow>
            <cylinderGeometry args={[0.12, 0.14, 2.1, 12]} />
            <meshStandardMaterial color={COLOR.stoneLight} roughness={0.88} />
          </mesh>
          <mesh position={[0, 2.15, 0]}>
            <boxGeometry args={[0.32, 0.12, 0.32]} />
            <meshStandardMaterial color={COLOR.stone} roughness={0.9} />
          </mesh>
          <mesh position={[0, 0.08, 0]}>
            <boxGeometry args={[0.34, 0.14, 0.34]} />
            <meshStandardMaterial color={COLOR.stoneDark} roughness={0.92} />
          </mesh>
          {/* wall sconce */}
          <mesh position={[x > 0 ? -0.18 : 0.18, 1.45, 0.12]}>
            <boxGeometry args={[0.08, 0.06, 0.1]} />
            <meshStandardMaterial color={COLOR.brassDim} metalness={0.7} roughness={0.35} />
          </mesh>
          <mesh position={[x > 0 ? -0.18 : 0.18, 1.55, 0.16]}>
            <sphereGeometry args={[0.045, 10, 10]} />
            <meshStandardMaterial color={COLOR.flame} emissive={COLOR.flame} emissiveIntensity={1.6} />
          </mesh>
        </group>
      ))}
      {/* Curtain panels */}
      <mesh position={[-1.7, 1.2, -1.35]} rotation={[0, 0.45, 0]}>
        <planeGeometry args={[0.9, 1.8]} />
        <meshStandardMaterial color={COLOR.curtain} roughness={0.95} side={THREE.DoubleSide} />
      </mesh>
      <mesh position={[1.7, 1.2, -1.35]} rotation={[0, -0.45, 0]}>
        <planeGeometry args={[0.9, 1.8]} />
        <meshStandardMaterial color={COLOR.curtain} roughness={0.95} side={THREE.DoubleSide} />
      </mesh>
      {/* Soft radial glow plate behind character */}
      <mesh position={[0, 0.85, -1.2]}>
        <circleGeometry args={[1.15, 32]} />
        <meshBasicMaterial
          color={combatActive ? COLOR.dangerGlow : COLOR.brassGlow}
          transparent
          opacity={0.07}
          depthWrite={false}
        />
      </mesh>
    </group>
  );
}

function OrnateRim() {
  return (
    <group position={[0, 0.01, 0]}>
      <mesh rotation={[-Math.PI / 2, 0, 0]}>
        <torusGeometry args={[0.92, 0.018, 10, 64]} />
        <meshStandardMaterial color={COLOR.brass} metalness={0.82} roughness={0.25} emissive={COLOR.brass} emissiveIntensity={0.12} />
      </mesh>
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, 0.004, 0]}>
        <torusGeometry args={[0.97, 0.008, 8, 64]} />
        <meshStandardMaterial color={COLOR.brassGlow} metalness={0.85} roughness={0.2} emissive={COLOR.brass} emissiveIntensity={0.18} />
      </mesh>
      {Array.from({ length: 12 }, (_, i) => {
        const a = (i / 12) * Math.PI * 2;
        return (
          <mesh key={i} position={[Math.cos(a) * 0.92, 0.015, Math.sin(a) * 0.92]}>
            <sphereGeometry args={[0.018, 10, 10]} />
            <meshStandardMaterial color={COLOR.brassGlow} metalness={0.9} roughness={0.18} emissive={COLOR.brass} emissiveIntensity={0.2} />
          </mesh>
        );
      })}
    </group>
  );
}

/** Game table in front of the DM — top aligned to waist height. */
function GameTable({ avatarScale }: { avatarScale: number }) {
  const { position, scale } = tablePlacement(avatarScale);
  return (
    <group position={position} scale={scale}>
      <mesh position={[0, 0, 0]} receiveShadow castShadow>
        <cylinderGeometry args={[0.72, 0.74, 0.055, 48]} />
        <meshStandardMaterial color={COLOR.wood} roughness={0.8} metalness={0.05} />
      </mesh>
      <mesh position={[0, 0.032, 0]} receiveShadow>
        <cylinderGeometry args={[0.62, 0.62, 0.012, 48]} />
        <meshStandardMaterial color={COLOR.felt} roughness={0.98} />
      </mesh>
      <OrnateRim />
    </group>
  );
}

function Candle({ reduced, avatarScale }: { reduced: boolean; avatarScale: number }) {
  const flame = useRef<THREE.Mesh>(null);
  useFrame((state) => {
    if (!flame.current || reduced) return;
    const t = state.clock.elapsedTime;
    const s = 1 + Math.sin(t * 12) * 0.14 + Math.sin(t * 21) * 0.07;
    flame.current.scale.set(s * 0.8, s, s * 0.8);
    flame.current.position.y = 0.22 + Math.sin(t * 9) * 0.01;
  });
  const { position, scale } = tablePlacement(avatarScale);
  const [tx, ty, tz] = position;
  return (
    <group position={[tx - 0.22 * scale, ty + 0.05 * scale, tz + 0.08 * scale]}>
      <mesh position={[0, 0.02, 0]} castShadow>
        <cylinderGeometry args={[0.045, 0.055, 0.03, 16]} />
        <meshStandardMaterial color={COLOR.brass} metalness={0.8} roughness={0.25} />
      </mesh>
      <mesh position={[0, 0.1, 0]} castShadow>
        <cylinderGeometry args={[0.026, 0.03, 0.13, 12]} />
        <meshStandardMaterial color="#f0e4c4" roughness={0.72} />
      </mesh>
      <mesh ref={flame} position={[0, 0.18, 0]}>
        <sphereGeometry args={[0.022, 12, 12]} />
        <meshStandardMaterial color={COLOR.flame} emissive={COLOR.flame} emissiveIntensity={2.4} transparent opacity={0.9} />
      </mesh>
      <mesh position={[0, 0.18, 0]}>
        <sphereGeometry args={[0.04, 10, 10]} />
        <meshStandardMaterial color={COLOR.flameCore} emissive={COLOR.flameCore} emissiveIntensity={1.1} transparent opacity={0.16} depthWrite={false} />
      </mesh>
    </group>
  );
}

function Props({ avatarScale }: { avatarScale: number }) {
  const { position, scale } = tablePlacement(avatarScale);
  return (
    <group position={position} scale={scale}>
      <group position={[0.34, 0.06, 0.02]} rotation={[0, -0.5, 0]}>
        <mesh castShadow position={[0, 0.02, 0]}>
          <boxGeometry args={[0.16, 0.035, 0.2]} />
          <meshStandardMaterial color="#4a3024" roughness={0.85} />
        </mesh>
        <mesh castShadow position={[0.01, 0.055, 0]} rotation={[0, 0.12, 0]}>
          <boxGeometry args={[0.14, 0.03, 0.18]} />
          <meshStandardMaterial color="#3a4030" roughness={0.8} />
        </mesh>
      </group>
      <mesh position={[-0.08, 0.06, 0.22]} rotation={[0.1, 0.6, 0.2]} castShadow>
        <cylinderGeometry args={[0.03, 0.03, 0.22, 12]} />
        <meshStandardMaterial color={COLOR.parchment} roughness={0.9} />
      </mesh>
    </group>
  );
}

function RollingDice({ rolling, checkPending, reduced, avatarScale }: { rolling: boolean; checkPending: boolean; reduced: boolean; avatarScale: number }) {
  const d20 = useRef<THREE.Mesh>(null);
  const d6 = useRef<THREE.Mesh>(null);
  const phase = useRef(0);
  const wasRolling = useRef(false);
  // Rest on the tabletop (world space, tracks waist-height table).
  const rest = useMemo(() => {
    const { position, scale } = tablePlacement(avatarScale);
    const [tx, ty, tz] = position;
    return {
      d20: new THREE.Vector3(tx + 0.14 * scale, ty + 0.09 * scale, tz + 0.06 * scale),
      d6: new THREE.Vector3(tx + 0.22 * scale, ty + 0.08 * scale, tz - 0.02 * scale),
    };
  }, [avatarScale]);

  useFrame((state, delta) => {
    const t = state.clock.elapsedTime;
    const a = d20.current;
    const b = d6.current;
    if (!a || !b) return;
    if (rolling && !wasRolling.current) phase.current = 0;
    wasRolling.current = rolling;
    if (rolling && !reduced) {
      phase.current += delta;
      const p = phase.current;
      const up = Math.max(0, Math.sin(Math.min(p, 0.85) * Math.PI)) * 0.55;
      const spin = p * 28;
      a.position.set(rest.d20.x + Math.sin(p * 9) * 0.12, rest.d20.y + up, rest.d20.z + Math.cos(p * 7) * 0.08);
      a.rotation.set(spin * 1.3, spin, spin * 0.7);
      b.position.set(rest.d6.x + Math.cos(p * 11) * 0.1, rest.d6.y + up * 0.85, rest.d6.z + Math.sin(p * 8) * 0.1);
      b.rotation.set(spin * 0.9, spin * 1.4, spin * 0.5);
      if (p > 0.85) {
        const land = Math.exp(-(p - 0.85) * 6) * Math.sin((p - 0.85) * 40) * 0.06;
        a.position.y = rest.d20.y + Math.abs(land);
        b.position.y = rest.d6.y + Math.abs(land) * 0.8;
      }
      return;
    }
    const hover = checkPending && !reduced ? 0.03 + Math.sin(t * 3.5) * 0.015 : 0;
    a.position.lerp(new THREE.Vector3(rest.d20.x, rest.d20.y + hover, rest.d20.z), 1 - Math.exp(-delta * 8));
    b.position.lerp(new THREE.Vector3(rest.d6.x, rest.d6.y + hover * 0.7, rest.d6.z), 1 - Math.exp(-delta * 8));
    if (!reduced) {
      a.rotation.y = checkPending ? t * 0.6 : t * 0.15;
      b.rotation.x = checkPending ? t * 0.5 : t * 0.12;
    }
    const mat = a.material as THREE.MeshStandardMaterial;
    mat.emissiveIntensity = rolling ? 0.55 : checkPending ? 0.35 + Math.sin(t * 4) * 0.12 : 0.18;
  });

  return (
    <group>
      <mesh ref={d20} position={rest.d20.toArray() as [number, number, number]} castShadow scale={0.85}>
        <icosahedronGeometry args={[0.07, 0]} />
        <meshStandardMaterial color={COLOR.brass} emissive={COLOR.brass} emissiveIntensity={0.18} metalness={0.5} roughness={0.25} />
      </mesh>
      <mesh ref={d6} position={rest.d6.toArray() as [number, number, number]} castShadow scale={0.85}>
        <boxGeometry args={[0.065, 0.065, 0.065]} />
        <meshStandardMaterial color="#6a5040" roughness={0.4} metalness={0.12} />
      </mesh>
    </group>
  );
}

function GltfFigure({ url, active, reduced, loop, scale }: { url: string; active: boolean; reduced: boolean; loop: boolean; scale: number }) {
  const root = useRef<THREE.Group>(null);
  const { scene, animations } = useGLTF(url);
  // SkeletonUtils.clone rebinds skin indices to the cloned armature. Plain
  // scene.clone(true) leaves SkinnedMesh.skeleton pointing at the cached
  // original bones — clips "play" but the mesh never deforms.
  const model = useMemo(() => cloneSkinned(scene), [scene]);
  // Drive clips from the skinned root itself so PropertyBinding finds Hips/…
  const { actions } = useAnimations(animations, model);
  const safeScale = clampDmAvatarScale(scale);

  useEffect(() => {
    model.traverse((obj) => {
      const mesh = obj as THREE.Mesh;
      if (mesh.isMesh) {
        mesh.castShadow = true;
        mesh.receiveShadow = true;
        const mats = Array.isArray(mesh.material) ? mesh.material : [mesh.material];
        mats.forEach((mat) => {
          if (mat) {
            mat.side = THREE.FrontSide;
            mat.needsUpdate = true;
          }
        });
      }
    });
  }, [model]);

  // Keep transform on the outer group only — never write scale onto the skinned
  // root, or the mixer/bindings can fight the pose.
  useEffect(() => {
    if (root.current) {
      root.current.scale.setScalar(safeScale);
      root.current.position.set(AVATAR.position[0], AVATAR.position[1], AVATAR.position[2]);
    }
  }, [safeScale]);

  useEffect(() => {
    const list = Object.values(actions).filter(Boolean) as THREE.AnimationAction[];
    if (list.length === 0) return;
    const action = list[0];
    if (!active) {
      action.fadeOut(0.2);
      return;
    }
    action.reset();
    action.enabled = true;
    action.setEffectiveWeight(1);
    action.setLoop(loop ? THREE.LoopRepeat : THREE.LoopOnce, loop ? Infinity : 1);
    action.clampWhenFinished = !loop;
    action.paused = !!reduced;
    if (reduced) {
      action.time = 0;
      action.play();
    } else {
      action.fadeIn(0.2).play();
    }
    return () => {
      action.fadeOut(0.15);
    };
  }, [actions, active, loop, reduced]);

  // useAnimations already advances the mixer each frame; only force outer scale/visibility here.
  useFrame(() => {
    if (root.current) {
      root.current.scale.setScalar(safeScale);
      root.current.visible = active;
    }
  });

  return (
    <group ref={root} visible={active} position={AVATAR.position} scale={safeScale}>
      <primitive object={model} />
    </group>
  );
}

function DungeonMasterAvatar({
  speaking,
  thinking,
  rolling,
  rollOutcome,
  reduced,
  avatarScale,
}: {
  speaking: boolean;
  thinking: boolean;
  rolling: boolean;
  rollOutcome: DiceAnimOutcome;
  reduced: boolean;
  avatarScale: number;
}) {
  const mode = resolveMode({ speaking, thinking, rolling, rollOutcome });
  // After a one-shot dice clip ends, fall back to idle visually via mode when rolling becomes false.
  return (
    <group>
      <GltfFigure
        key={mode}
        url={GLB[mode]}
        active
        reduced={reduced}
        loop={mode === 'idle' || mode === 'talk'}
        scale={avatarScale}
      />
    </group>
  );
}

export function DMTable({
  speaking,
  thinking,
  combatActive,
  rolling = false,
  rollOutcome = null,
  checkPending = false,
  scene,
  avatarScale = DM_AVATAR_SCALE_DEFAULT,
}: DMTableProps) {
  const reduced = usePrefersReducedMotion();
  const scale = clampDmAvatarScale(avatarScale);
  const table = tablePlacement(scale);
  // Orbit target: mid-chest / just above the tabletop.
  const lookAt = useMemo(
    (): [number, number, number] => [0, table.waistY * 0.92, LAYOUT.tableZ * 0.35],
    [table.waistY],
  );
  const label = scene ? `地城主肖像・${scene}` : '地城主肖像';
  const caption = rolling
    ? rollOutcome === 'success'
      ? '擲骰成功'
      : rollOutcome === 'fail'
        ? '擲骰失敗'
        : '擲骰'
    : thinking
      ? '裁定中'
      : speaking
        ? '敘事中'
        : checkPending
          ? '待擲骰'
          : combatActive
            ? '戰鬥'
            : '地城主';

  return (
    <div
      className="dm-portrait"
      role="img"
      aria-label={label}
      data-state={caption}
    >
      <Canvas
        shadows
        dpr={[1, 1.5]}
        camera={{ position: LAYOUT.cameraPos, fov: LAYOUT.fov, near: 0.1, far: 24 }}
        gl={{
          alpha: false,
          antialias: true,
          powerPreference: 'default',
          toneMapping: THREE.ACESFilmicToneMapping,
          toneMappingExposure: 1.12,
        }}
        frameloop={reduced ? 'demand' : 'always'}
        onCreated={({ gl, camera }) => {
          gl.shadowMap.enabled = true;
          gl.shadowMap.type = THREE.PCFSoftShadowMap;
          camera.lookAt(...lookAt);
        }}
      >
        <color attach="background" args={[COLOR.voidDeep]} />
        <InitialCamera lookAt={lookAt} />
        <PortraitControls target={lookAt} />
        <Lights combatActive={combatActive} rolling={rolling} reduced={reduced} />
        <ChamberBackground combatActive={combatActive} />
        {/* Avatar on floor; table at waist height in front */}
        <DungeonMasterAvatar
          speaking={speaking}
          thinking={thinking}
          rolling={rolling}
          rollOutcome={rollOutcome}
          reduced={reduced}
          avatarScale={scale}
        />
        <GameTable avatarScale={scale} />
        <Candle reduced={reduced} avatarScale={scale} />
        <Props avatarScale={scale} />
        <RollingDice rolling={rolling} checkPending={checkPending} reduced={reduced} avatarScale={scale} />
        {!reduced && (
          <Sparkles
            count={rolling ? 28 : 14}
            scale={[1.2, 1.5, 1.0]}
            size={rolling ? 2.0 : 1.25}
            speed={rolling ? 0.7 : 0.18}
            opacity={0.28}
            color={rolling ? COLOR.brassGlow : combatActive ? COLOR.dangerGlow : COLOR.brassGlow}
            position={[LAYOUT.avatar[0], table.waistY, LAYOUT.avatar[2]]}
          />
        )}
        <ContactShadows position={[LAYOUT.avatar[0], 0.005, LAYOUT.avatar[2]]} opacity={0.5} blur={2.6} scale={3.5} far={3} color="#000000" />
      </Canvas>
      <div className="dm-portrait-vignette" aria-hidden="true" />
      <span className="dm-portrait-caption" aria-hidden="true">{caption}</span>
    </div>
  );
}

export default DMTable;
