import { useState } from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { colors, shared } from '@/src/theme';

export default function ScoringScreen() {
  const { client } = useSession();
  const [namesInput, setNamesInput] = useState('Alice,Bob,Carol,Dave');
  const [sessionId, setSessionId] = useState('');
  const [players, setPlayers] = useState<string[]>([]);
  const [round, setRound] = useState(1);
  const [scoresInput, setScoresInput] = useState('');
  const [exportText, setExportText] = useState('');
  const [error, setError] = useState('');

  async function createSession() {
    setError('');
    const names = namesInput
      .split(',')
      .map((n) => n.trim())
      .filter(Boolean);
    if (names.length < 4 || names.length > 8) {
      setError('Enter 4–8 comma-separated player names');
      return;
    }
    try {
      const id = await client.createScoringSession(names);
      setSessionId(id);
      setPlayers(names);
      setExportText('');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Create failed');
    }
  }

  async function saveRound() {
    if (!sessionId) return;
    setError('');
    const scores: Record<string, number> = {};
    for (const part of scoresInput.split(',')) {
      const [name, val] = part.split(':').map((s) => s.trim());
      if (name && val) {
        scores[name] = parseInt(val, 10) || 0;
      }
    }
    if (Object.keys(scores).length === 0) {
      setError('Scores format: Name:100,Name2:50');
      return;
    }
    try {
      await client.patchScoringSession(sessionId, round, scores);
      setRound((r) => r + 1);
      setScoresInput('');
      const data = await client.getScoringSession(sessionId);
      setExportText(JSON.stringify(data, null, 2));
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Save failed');
    }
  }

  async function doExport() {
    if (!sessionId) return;
    setError('');
    try {
      const text = await client.exportScoringSession(sessionId);
      setExportText(text);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Export failed');
    }
  }

  return (
    <Screen title="Offline score table" scroll>
      {!sessionId ? (
        <>
          <Text style={shared.status}>Comma-separated names (4–8 players)</Text>
          <TextInput
            style={shared.input}
            value={namesInput}
            onChangeText={setNamesInput}
            placeholder="P1,P2,P3,P4"
            placeholderTextColor="#8b9cb3"
          />
          <Pressable style={shared.button} onPress={createSession}>
            <Text style={shared.buttonText}>New session</Text>
          </Pressable>
        </>
      ) : (
        <View>
          <Text style={{ color: colors.text }}>Session: {sessionId}</Text>
          <Text style={shared.status}>Players: {players.join(', ')}</Text>
          <Text style={[shared.status, { marginTop: 12 }]}>Round {round} scores</Text>
          <TextInput
            style={shared.input}
            value={scoresInput}
            onChangeText={setScoresInput}
            placeholder="Alice:120,Bob:80,…"
            placeholderTextColor="#8b9cb3"
          />
          <Pressable style={shared.button} onPress={saveRound}>
            <Text style={shared.buttonText}>Save round</Text>
          </Pressable>
          <Pressable style={[shared.button, shared.buttonSecondary]} onPress={doExport}>
            <Text style={shared.buttonTextSecondary}>Export scorecard</Text>
          </Pressable>
        </View>
      )}
      {error ? <Text style={shared.error}>{error}</Text> : null}
      {exportText ? (
        <Text style={[shared.status, { marginTop: 12, fontFamily: 'monospace' }]}>
          {exportText}
        </Text>
      ) : null}
    </Screen>
  );
}
