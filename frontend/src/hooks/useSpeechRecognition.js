import { useState, useEffect, useRef, useCallback } from 'react';

/**
 * Hook for Web Speech API speech recognition
 * @param {Object} options
 * @param {Function} options.onResult - Callback when speech is recognized
 * @param {Function} options.onError - Callback when an error occurs
 * @param {string} options.lang - Language code (default: 'es-ES')
 * @param {boolean} options.continuous - Keep listening after results (default: true)
 * @param {boolean} options.interimResults - Return interim results (default: true)
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
  const onResultRef = useRef(onResult);
  const onErrorRef = useRef(onError);

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
    recognition.continuous = continuous;
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

      if (onErrorRef.current) {
        onErrorRef.current(errorMessage);
      }
    };

    recognition.onend = () => {
      setIsListening(false);
    };

    recognitionRef.current = recognition;

    return () => {
      recognition.abort();
    };
  }, [isSupported, lang, continuous, interimResults]);

  const startListening = useCallback(() => {
    if (!recognitionRef.current || isListening) return;

    setError(null);
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
