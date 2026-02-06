import { useState, useEffect, useRef, useCallback } from 'react';

const isMobile = () =>
  typeof navigator !== 'undefined' && /Android|iPhone|iPad|iPod/i.test(navigator.userAgent);

/**
 * Hook for Web Speech API speech recognition.
 *
 * On mobile (Android Chrome especially) continuous mode causes duplicate/
 * triplicate words due to a known Chromium bug (crbug.com/40324711).
 * We force continuous=false on mobile and re-start automatically after each
 * final result so the UX stays seamless (push-to-talk style).
 */
export function useSpeechRecognition({
  onResult,
  onError,
  lang = 'es-ES',
  continuous = true,
  interimResults = true,
} = {}) {
  const [isListening, setIsListening] = useState(false);
  const [error, setError] = useState(null);
  const recognitionRef = useRef(null);
  const wantListeningRef = useRef(false);
  const onResultRef = useRef(onResult);
  const onErrorRef = useRef(onError);

  const mobile = isMobile();
  // Force continuous off on mobile to avoid the duplicate-words bug
  const effectiveContinuous = mobile ? false : continuous;

  // Keep refs updated
  useEffect(() => {
    onResultRef.current = onResult;
    onErrorRef.current = onError;
  }, [onResult, onError]);

  // Check browser support
  const isSupported = typeof window !== 'undefined' &&
    ('SpeechRecognition' in window || 'webkitSpeechRecognition' in window);

  // Initialize recognition
  useEffect(() => {
    if (!isSupported) return;

    const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;
    const recognition = new SpeechRecognition();

    recognition.lang = lang;
    recognition.continuous = effectiveContinuous;
    recognition.interimResults = interimResults;

    recognition.onresult = (event) => {
      const lastResult = event.results[event.results.length - 1];
      const transcript = lastResult[0].transcript;
      const isFinal = lastResult.isFinal;

      if (isFinal && onResultRef.current) {
        onResultRef.current(transcript.trim());
      }
    };

    recognition.onerror = (event) => {
      let errorMessage;
      switch (event.error) {
        case 'not-allowed':
          errorMessage = 'Permiso de micrófono denegado';
          break;
        case 'no-speech':
          errorMessage = 'No se detectó voz';
          break;
        case 'audio-capture':
          errorMessage = 'No se encontró micrófono';
          break;
        case 'network':
          errorMessage = 'Error de red';
          break;
        case 'aborted':
          // User stopped, not an error
          return;
        default:
          errorMessage = `Error: ${event.error}`;
      }

      setError(errorMessage);
      setIsListening(false);
      wantListeningRef.current = false;

      if (onErrorRef.current) {
        onErrorRef.current(errorMessage);
      }
    };

    recognition.onend = () => {
      // On mobile (non-continuous), auto-restart if user hasn't stopped
      if (!effectiveContinuous && wantListeningRef.current) {
        try {
          recognition.start();
          return; // keep isListening true
        } catch {
          // fall through to stop
        }
      }
      setIsListening(false);
      wantListeningRef.current = false;
    };

    recognitionRef.current = recognition;

    return () => {
      wantListeningRef.current = false;
      recognition.abort();
    };
  }, [isSupported, lang, effectiveContinuous, interimResults]);

  const startListening = useCallback(() => {
    if (!recognitionRef.current || isListening) return;

    setError(null);
    wantListeningRef.current = true;
    try {
      recognitionRef.current.start();
      setIsListening(true);
    } catch (err) {
      // Recognition may already be started
      console.warn('Speech recognition start error:', err);
    }
  }, [isListening]);

  const stopListening = useCallback(() => {
    if (!recognitionRef.current || !isListening) return;

    wantListeningRef.current = false;
    try {
      recognitionRef.current.stop();
    } catch (err) {
      console.warn('Speech recognition stop error:', err);
    }
    setIsListening(false);
  }, [isListening]);

  const toggleListening = useCallback(() => {
    if (isListening) {
      stopListening();
    } else {
      startListening();
    }
  }, [isListening, startListening, stopListening]);

  return {
    isListening,
    isSupported,
    error,
    startListening,
    stopListening,
    toggleListening,
  };
}

export default useSpeechRecognition;
