import { useMemo } from 'react';
import ReactFlow, {
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  MarkerType,
} from 'reactflow';
import 'reactflow/dist/style.css';
import { getStatusColor } from '../lib/theme';

const nodeWidth = 180;
const nodeHeight = 50;

// Simple auto-layout algorithm
const getLayoutedElements = (tasks) => {
  const nodes = [];
  const edges = [];
  const levelMap = {};
  const processed = new Set();

  // 1. Build adjacency list and in-degree
  const adj = {};
  const inDegree = {};
  tasks.forEach(t => {
    adj[t.id] = [];
    inDegree[t.id] = 0;
  });

  tasks.forEach(t => {
    if (t.dependencies) {
      t.dependencies.forEach(depId => {
        if (adj[depId]) adj[depId].push(t.id);
        inDegree[t.id] = (inDegree[t.id] || 0) + 1;
        
        edges.push({
          id: `${depId}-${t.id}`,
          source: depId,
          target: t.id,
          type: 'smoothstep',
          animated: true,
          markerEnd: { type: MarkerType.ArrowClosed },
          style: { stroke: 'var(--border-color)' },
        });
      });
    }
  });

  // 2. Assign levels (Topo sort / BFS)
  const queue = tasks.filter(t => inDegree[t.id] === 0).map(t => ({ id: t.id, level: 0 }));
  let maxLevel = 0;

  while (queue.length > 0) {
    const { id, level } = queue.shift();
    if (processed.has(id)) continue;
    processed.add(id);
    
    levelMap[id] = level;
    maxLevel = Math.max(maxLevel, level);

    if (adj[id]) {
      adj[id].forEach(childId => {
        inDegree[childId]--;
        if (inDegree[childId] === 0) {
          queue.push({ id: childId, level: level + 1 });
        }
      });
    }
  }

  // Handle cycles or disconnected (orphan) nodes by putting them at maxLevel + 1 or 0
  tasks.forEach(t => {
    if (levelMap[t.id] === undefined) levelMap[t.id] = 0;
  });

  // 3. Position nodes
  const nodesByLevel = {};
  tasks.forEach(t => {
    const lvl = levelMap[t.id];
    if (!nodesByLevel[lvl]) nodesByLevel[lvl] = [];
    nodesByLevel[lvl].push(t);
  });

  Object.entries(nodesByLevel).forEach(([lvl, levelTasks]) => {
    levelTasks.forEach((task, index) => {
      const { bg, text } = getStatusColor(task.status);
      nodes.push({
        id: task.id,
        data: { label: task.name || task.id, status: task.status, bg, text },
        position: { x: index * (nodeWidth + 50), y: Number(lvl) * (nodeHeight + 100) },
        style: { 
            width: nodeWidth, 
            padding: '10px', 
            borderRadius: '8px', 
            border: '1px solid var(--border-color)',
            background: 'var(--card-bg)',
            color: 'var(--foreground)',
            fontSize: '12px',
            textAlign: 'center',
        },
      });
    });
  });

  return { nodes, edges };
};

export default function WorkflowGraph({ tasks }) {
  const { nodes: initialNodes, edges: initialEdges } = useMemo(() => getLayoutedElements(tasks), [tasks]);
  
  // We use useNodesState/useEdgesState even if read-only to handle internal interactions
  const [nodes, , onNodesChange] = useNodesState(initialNodes);
  const [edges, , onEdgesChange] = useEdgesState(initialEdges);

  // Update nodes when tasks change (simple reset)
  // In a real app we might want to preserve positions if user dragged them, 
  // but for now re-layout on task change is safer.
  
  return (
    <div className="h-[500px] border border-border rounded-xl bg-card/50 overflow-hidden">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        fitView
        attributionPosition="bottom-right"
      >
        <Controls />
        <Background color="#aaa" gap={16} />
      </ReactFlow>
    </div>
  );
}
